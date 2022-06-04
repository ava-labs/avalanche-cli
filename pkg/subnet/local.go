// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/utils"
	avago_genesis "github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/ava-labs/coreth/core"
	"github.com/ava-labs/coreth/params"
)

type SubnetDeployer struct {
	procChecker         binutils.ProcessChecker
	binChecker          binutils.BinaryChecker
	getClientFunc       getGRPCClientFunc
	binaryDownloader    binutils.PluginBinaryDownloader
	healthCheckInterval time.Duration
	log                 logging.Logger
	baseDir             string
	backendStartedHere  bool
}

func NewLocalSubnetDeployer(log logging.Logger, baseDir string) *SubnetDeployer {
	return &SubnetDeployer{
		procChecker:         binutils.NewProcessChecker(),
		binChecker:          binutils.NewBinaryChecker(),
		getClientFunc:       binutils.NewGRPCClient,
		binaryDownloader:    binutils.NewPluginBinaryDownloader(log),
		healthCheckInterval: 100 * time.Millisecond,
		log:                 log,
		baseDir:             baseDir,
	}
}

type getGRPCClientFunc func() (client.Client, error)

// DeployToLocalNetwork does the heavy lifting:
// * it checks the gRPC is running, if not, it starts it
// * kicks off the actual deployment
func (d *SubnetDeployer) DeployToLocalNetwork(chain string, chain_genesis string) error {
	if err := d.StartServer(); err != nil {
		return err
	}
	return d.doDeploy(chain, chain_genesis)
}

func (d *SubnetDeployer) StartServer() error {
	isRunning, err := d.procChecker.IsServerProcessRunning()
	if err != nil {
		return fmt.Errorf("failed querying if server process is running: %w", err)
	}
	if !isRunning {
		d.log.Debug("gRPC server is not running")
		if err := binutils.StartServerProcess(d.log); err != nil {
			return fmt.Errorf("failed starting gRPC server process: %w", err)
		}
		d.backendStartedHere = true
	}
	return nil
}

// BackendStartedHere returns true if the backend was started by this run,
// or false if it found it there already
func (d *SubnetDeployer) BackendStartedHere() bool {
	return d.backendStartedHere
}

func (d *SubnetDeployer) GetBinPaths() (string, string, error) {
	avagoDir, err := d.setupLocalEnv()
	if err != nil {
		return "", "", fmt.Errorf("failed setting up local environment: %w", err)
	}

	pluginDir := filepath.Join(avagoDir, "plugins")
	avalancheGoBinPath := filepath.Join(avagoDir, "avalanchego")

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return "", "", fmt.Errorf("evaluated pluginDir to be %s but it does not exist.", pluginDir)
	}

	// TODO: we need some better version management here
	// * compare latest to local version
	// * decide if force update or give user choice
	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return "", "", fmt.Errorf("evaluated avalancheGoBinPath to be %s but it does not exist.", avalancheGoBinPath)
	}

	return avalancheGoBinPath, pluginDir, nil
}

// doDeploy the actual deployment to the network runner
func (d *SubnetDeployer) doDeploy(chain string, chain_genesis string) error {

	avalancheGoBinPath, pluginDir, err := d.GetBinPaths()
	if err != nil {
		return err
	}

	cli, err := d.getClientFunc()
	if err != nil {
		return fmt.Errorf("error creating gRPC Client: %s", err)
	}
	defer cli.Close()

	exists, err := storage.FileExists(chain_genesis)
	if !exists || err != nil {
		return fmt.Errorf(
			"evaluated chain genesis file to be at %s but it does not seem to exist.", chain_genesis)
	}

	// we need the chainID just later, but it would be ugly to fail the whole deployment
	// for a JSON unmarshalling error, so let's do it here already
	genesis, err := getGenesis(chain_genesis)
	if err != nil {
		return fmt.Errorf("failed to unpack chain ID from genesis: %w", err)
	}
	chainID := genesis.Config.ChainID

	vmID, err := utils.VMID(chain)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	d.log.Debug("this VM will get ID: %s", vmID.String())

	binDir := filepath.Join(d.baseDir, constants.AvalancheCliBinDir)
	if err := d.binaryDownloader.Download(vmID, pluginDir, binDir); err != nil {
		return err
	}

	ctx := binutils.GetAsyncContext()

	ux.Logger.PrintToUser("VM ready. Trying to boot network...")
	loadSnapshotOpts := []client.OpOption{
		client.WithPluginDir(pluginDir),
		client.WithExecPath(avalancheGoBinPath),
	}
	loadSnapshotsInfo, err := cli.LoadSnapshot(
		ctx,
		constants.DefaultSnapshotName,
		loadSnapshotOpts...,
	)
	if err != nil {
		if !strings.Contains(err.Error(), "already bootstrapped") {
			return fmt.Errorf("failed to start network :%s", err)
		}
		ux.Logger.PrintToUser("Network has already been booted. Wait until healthy...")
	} else {
		ux.Logger.PrintToUser("Booting Network. Wait until healthy...")
	}

	d.log.Debug(loadSnapshotsInfo.String())

	subnetIDs, blockchainsInfo, _, err := d.WaitForHealthy(ctx, cli, d.healthCheckInterval)
	if err != nil {
		_ = binutils.KillgRPCServerProcess()
		return fmt.Errorf("failed to query network health: %s", err)
	}

	// choose next subnetID
	sort.Strings(subnetIDs)
	subnetId := subnetIDs[len(blockchainsInfo)%len(subnetIDs)]
	fmt.Printf("USANDO %s\n", subnetId)

	blockchainSpecs := []*rpcpb.BlockchainSpec{
		{
			VmName:   chain,
			Genesis:  chain_genesis,
			SubnetId: &subnetId,
		},
	}
	deployBlockchainsOpts := []client.OpOption{
		client.WithPluginDir(pluginDir),
		client.WithBlockchainSpecs(blockchainSpecs),
	}

	deployBlockchainsInfo, err := cli.DeployBlockchains(
		ctx,
		deployBlockchainsOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to deploy blockchain :%s", err)
	}

	d.log.Debug(deployBlockchainsInfo.String())

	fmt.Println()
	ux.Logger.PrintToUser("Blockchain has been deployed. Wail until network acknowledges...")

	_, _, endpoints, err := d.WaitForHealthy(ctx, cli, d.healthCheckInterval)
	if err != nil {
		_ = binutils.KillgRPCServerProcess()
		return fmt.Errorf("failed to query network health: %s", err)
	}

	fmt.Println()
	ux.Logger.PrintToUser("Network ready to use. Local network node endpoints:")
	for _, u := range endpoints {
		ux.Logger.PrintToUser(u)
	}
	fmt.Println()
	firstURL := endpoints[0]

	ux.Logger.PrintToUser("Metamask connection details (any node URL from above works):")
	ux.Logger.PrintToUser("RPC URL:          %s", firstURL[strings.LastIndex(firstURL, "http"):])
	for address := range genesis.Alloc {
		amount := genesis.Alloc[address].Balance
		formattedAmount := new(big.Int).Div(amount, big.NewInt(params.Ether))
		if address == vm.Prefunded_ewoq_Address {
			ux.Logger.PrintToUser("Funded address:   %s with %s (10^18) - private key: %s", address, formattedAmount.String(), avago_genesis.EWOQKeyStr)
		} else {
			ux.Logger.PrintToUser("Funded address:   %s with %s", address, formattedAmount.String())
		}
	}

	ux.Logger.PrintToUser("Network name:     %s", chain)
	ux.Logger.PrintToUser("Chain ID:         %s", chainID)
	ux.Logger.PrintToUser("Currency Symbol:  TEST")
	return nil
}

// setupLocalEnv also does some heavy lifting:
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads it and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func (d *SubnetDeployer) setupLocalEnv() (string, error) {
	binDir := filepath.Join(d.baseDir, constants.AvalancheCliBinDir)
	binPrefix := "avalanchego-v"

	exists, latest, err := d.binChecker.ExistsWithLatestVersion(binDir, binPrefix)
	if err != nil {
		return "", fmt.Errorf("failed trying to locate avalanchego binary: %s", binDir)
	}
	if exists {
		d.log.Debug("local avalanchego found. skipping installation")
		return latest, nil
	}

	ux.Logger.PrintToUser("Installing latest avalanchego version...")

	version, err := binutils.GetLatestReleaseVersion(constants.LatestAvagoReleaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to get latest avalanchego version: %s", err)
	}

	d.log.Info("Latest avalanchego version is: %s", version)

	// TODO: would be nice if we could also here just use binutils.DownloadLatestReleaseVersion(),
	// but unfortunately we don't have a consistent naming scheme between avalanchego and subnet-evm
	// releases and names (and supported `goos`).
	// Doing so therefore would require adding some questionable complexity.
	// The goal MUST be to have some sort of mature binary management

	// NOTE: if any of the underlying URLs change (github changes, release file names, etc.) this fails
	arch := runtime.GOARCH
	goos := runtime.GOOS
	var avalanchegoURL string
	var ext string

	switch goos {
	case "linux":
		avalanchegoURL = fmt.Sprintf(
			"https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-linux-%s-%s.tar.gz",
			version,
			arch,
			version,
		)
		ext = "tar.gz"
	case "darwin":
		avalanchegoURL = fmt.Sprintf(
			"https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-macos-%s.zip",
			version,
			version,
		)
		ext = "zip"
		// EXPERMENTAL WIN, no support
	case "windows":
		avalanchegoURL = fmt.Sprintf(
			"https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-win-%s-experimental.zip",
			version,
			version,
		)
		ext = "zip"
	default:
		return "", fmt.Errorf("OS not supported: %s", goos)
	}

	d.log.Debug("starting download from %s...", avalanchegoURL)

	resp, err := http.Get(avalanchegoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	archive, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	d.log.Debug("download successful. installing archive...")
	if err := binutils.InstallArchive(ext, archive, binDir); err != nil {
		return "", err
	}
	avagoSubDir := "avalanchego-" + version
	if ext == "zip" {
		// zip contains a build subdir instead of the avagoSubDir expected from tar.gz
		if err := os.Rename(filepath.Join(binDir, "build"), filepath.Join(binDir, avagoSubDir)); err != nil {
			return "", err
		}
	}
	ux.Logger.PrintToUser("Avalanchego installation successful")
	return filepath.Join(binDir, avagoSubDir), nil
}

// WaitForHealthy polls continuously until the network is ready to be used
func (d *SubnetDeployer) WaitForHealthy(
	ctx context.Context,
	cli client.Client,
	healthCheckInterval time.Duration,
) ([]string, []string, []string, error) {
	cancel := make(chan struct{})
	go ux.PrintWait(cancel)
	for {
		select {
		case <-ctx.Done():
			cancel <- struct{}{}
			return nil, nil, nil, ctx.Err()
		case <-time.After(healthCheckInterval):
			cancel <- struct{}{}
			d.log.Debug("polling for health...")
			go ux.PrintWait(cancel)
			resp, err := cli.Health(ctx)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("the health check failed to complete. The server might be down or have crashed, check the logs! %s", err)
			}
			if resp.ClusterInfo == nil {
				d.log.Debug("warning: ClusterInfo is nil. trying again...")
				continue
			}
			if !resp.ClusterInfo.Healthy {
				d.log.Debug("network is not healthy. polling again...")
				continue
			}
			if !resp.ClusterInfo.CustomVmsHealthy {
				d.log.Debug("network is up but custom VMs are not healthy. polling again...")
				continue
			}
			d.log.Debug("network is up and custom VMs are up")
			blockchains := []string{}
			for vmID, vmInfo := range resp.ClusterInfo.CustomVms {
				blockchains = append(blockchains, fmt.Sprintf("%s/%s/%s", vmID, vmInfo.BlockchainId, vmInfo.SubnetId))
			}
			endpoints := []string{}
			for _, nodeInfo := range resp.ClusterInfo.NodeInfos {
				for vmID, vmInfo := range resp.ClusterInfo.CustomVms {
					endpoints = append(endpoints, fmt.Sprintf("Endpoint at node %s for blockchain %q: %s/ext/bc/%s/rpc", nodeInfo.Name, vmID, nodeInfo.GetUri(), vmInfo.BlockchainId))
				}
			}
			close(cancel)
			return resp.ClusterInfo.Subnets, blockchains, endpoints, nil
		}
	}
}

// getGenesis extracts the chain genesis from the provided genesis file
// we don't need to check the existence of the file as we already did before
// TODO: We should probably store this in some global object when asking the user so we don't need
// to unpack this here anymore. The sidecar seems the best candidate
func getGenesis(genesisFile string) (core.Genesis, error) {
	var genesis core.Genesis
	genBytes, err := os.ReadFile(genesisFile)
	if err != nil {
		return genesis, err
	}
	if err := json.Unmarshal(genBytes, &genesis); err != nil {
		return genesis, err
	}
	return genesis, nil
}
