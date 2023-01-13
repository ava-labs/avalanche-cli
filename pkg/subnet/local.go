// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/server"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/ava-labs/coreth/params"
	spacesvmchain "github.com/ava-labs/spacesvm/chain"
	"github.com/ava-labs/subnet-evm/core"
	"go.uber.org/zap"
)

const (
	WriteReadReadPerms = 0o644
)

type LocalDeployer struct {
	procChecker         binutils.ProcessChecker
	binChecker          binutils.BinaryChecker
	getClientFunc       getGRPCClientFunc
	binaryDownloader    binutils.PluginBinaryDownloader
	healthCheckInterval time.Duration
	app                 *application.Avalanche
	backendStartedHere  bool
	setDefaultSnapshot  setDefaultSnapshotFunc
	avagoVersion        string
	vmBin               string
}

func NewLocalDeployer(app *application.Avalanche, avagoVersion string, vmBin string) *LocalDeployer {
	return &LocalDeployer{
		procChecker:         binutils.NewProcessChecker(),
		binChecker:          binutils.NewBinaryChecker(),
		getClientFunc:       binutils.NewGRPCClient,
		binaryDownloader:    binutils.NewPluginBinaryDownloader(app),
		healthCheckInterval: 100 * time.Millisecond,
		app:                 app,
		setDefaultSnapshot:  SetDefaultSnapshot,
		avagoVersion:        avagoVersion,
		vmBin:               vmBin,
	}
}

type getGRPCClientFunc func() (client.Client, error)

type setDefaultSnapshotFunc func(string, bool) error

// DeployToLocalNetwork does the heavy lifting:
// * it checks the gRPC is running, if not, it starts it
// * kicks off the actual deployment
func (d *LocalDeployer) DeployToLocalNetwork(chain string, chainGenesis []byte, genesisPath string) (ids.ID, ids.ID, error) {
	if err := d.StartServer(); err != nil {
		return ids.Empty, ids.Empty, err
	}
	return d.doDeploy(chain, chainGenesis, genesisPath)
}

func (d *LocalDeployer) StartServer() error {
	isRunning, err := d.procChecker.IsServerProcessRunning(d.app)
	if err != nil {
		return fmt.Errorf("failed querying if server process is running: %w", err)
	}
	if !isRunning {
		d.app.Log.Debug("gRPC server is not running")
		if err := binutils.StartServerProcess(d.app); err != nil {
			return fmt.Errorf("failed starting gRPC server process: %w", err)
		}
		d.backendStartedHere = true
	}
	return nil
}

// BackendStartedHere returns true if the backend was started by this run,
// or false if it found it there already
func (d *LocalDeployer) BackendStartedHere() bool {
	return d.backendStartedHere
}

// doDeploy the actual deployment to the network runner
// steps:
//   - checks if the network has been started
//   - install all needed plugin binaries, for the the new VM, and the already deployed VMs
//   - either starts a network from the default snapshot if not started,
//     or restarts the already available network while preserving state
//   - waits completion of operation
//   - get from the network an available subnet ID to be used in blockchain creation
//   - deploy a new blockchain for the given VM ID, genesis, and available subnet ID
//   - waits completion of operation
//   - show status
func (d *LocalDeployer) doDeploy(chain string, chainGenesis []byte, genesisPath string) (ids.ID, ids.ID, error) {
	avagoVersion, avalancheGoBinPath, pluginDir, err := d.SetupLocalEnv()
	if err != nil {
		return ids.Empty, ids.Empty, err
	}

	cli, err := d.getClientFunc()
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("error creating gRPC Client: %w", err)
	}
	defer cli.Close()

	runDir := d.app.GetRunDir()

	ctx := binutils.GetAsyncContext()

	// check for network and get VM info
	networkBooted := true
	clusterInfo, err := d.WaitForHealthy(ctx, cli, d.healthCheckInterval)
	if err != nil {
		if !server.IsServerError(err, server.ErrNotBootstrapped) {
			return ids.Empty, ids.Empty, fmt.Errorf("failed to query network health: %w", err)
		} else {
			networkBooted = false
		}
	}

	chainVMID, err := anrutils.VMID(chain)
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	d.app.Log.Debug("this VM will get ID", zap.String("vm-id", chainVMID.String()))

	if alreadyDeployed(chainVMID, clusterInfo) {
		ux.Logger.PrintToUser("Subnet %s has already been deployed", chain)
		return ids.Empty, ids.Empty, nil
	}

	if err := d.installPlugin(chainVMID, d.vmBin, pluginDir); err != nil {
		return ids.Empty, ids.Empty, err
	}

	ux.Logger.PrintToUser("VMs ready.")

	if !networkBooted {
		if err := d.startNetwork(ctx, cli, avagoVersion, avalancheGoBinPath, pluginDir, runDir); err != nil {
			return ids.Empty, ids.Empty, err
		}
	}

	clusterInfo, err = d.WaitForHealthy(ctx, cli, d.healthCheckInterval)
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("failed to query network health: %w", err)
	}
	subnetIDs := clusterInfo.Subnets
	numBlockchains := len(clusterInfo.CustomChains)

	// in order to make subnet deploy faster, a set of validated subnet IDs is preloaded
	// in the bootstrap snapshot
	// we select one to be used for creating the next blockchain, for that we use the
	// number of currently created blockchains as the index to select the next subnet ID,
	// so we get incremental selection
	sort.Strings(subnetIDs)
	if len(subnetIDs) == 0 {
		return ids.Empty, ids.Empty, errors.New("the network has not preloaded subnet IDs")
	}
	subnetIDStr := subnetIDs[numBlockchains%len(subnetIDs)]

	// if a chainConfig has been configured
	var (
		chainConfig            string
		chainConfigFile        = filepath.Join(d.app.GetSubnetDir(), chain, constants.ChainConfigFileName)
		perNodeChainConfig     string
		perNodeChainConfigFile = filepath.Join(d.app.GetSubnetDir(), chain, constants.PerNodeChainConfigFileName)
	)
	if _, err := os.Stat(chainConfigFile); err == nil {
		// currently the ANR only accepts the file as a path, not its content
		chainConfig = chainConfigFile
	}
	if _, err := os.Stat(perNodeChainConfigFile); err == nil {
		perNodeChainConfig = perNodeChainConfigFile
	}
	// create a new blockchain on the already started network, associated to
	// the given VM ID, genesis, and available subnet ID
	blockchainSpecs := []*rpcpb.BlockchainSpec{
		{
			VmName:             chain,
			Genesis:            genesisPath,
			SubnetId:           &subnetIDStr,
			ChainConfig:        chainConfig,
			PerNodeChainConfig: perNodeChainConfig,
		},
	}
	deployBlockchainsInfo, err := cli.CreateBlockchains(
		ctx,
		blockchainSpecs,
	)
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("failed to deploy blockchain: %w", err)
	}

	d.app.Log.Debug(deployBlockchainsInfo.String())

	fmt.Println()
	ux.Logger.PrintToUser("Blockchain has been deployed. Wait until network acknowledges...")

	clusterInfo, err = d.WaitForHealthy(ctx, cli, d.healthCheckInterval)
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("failed to query network health: %w", err)
	}

	endpoints := GetEndpoints(clusterInfo)

	fmt.Println()
	ux.Logger.PrintToUser("Network ready to use. Local network node endpoints:")
	ux.PrintTableEndpoints(clusterInfo)
	fmt.Println()

	firstURL := endpoints[0]

	ux.Logger.PrintToUser("Browser Extension connection details (any node URL from above works):")
	ux.Logger.PrintToUser("RPC URL:          %s", firstURL[strings.LastIndex(firstURL, "http"):])

	// extra ux based on vm type
	sc, err := d.app.LoadSidecar(chain)
	if err != nil {
		return ids.Empty, ids.Empty, fmt.Errorf("failed to load sidecar: %w", err)
	}
	switch sc.VM {
	case models.SubnetEvm:
		if err := d.printExtraEvmInfo(chain, chainGenesis); err != nil {
			// not supposed to happen due to genesis pre validation
			return ids.Empty, ids.Empty, nil
		}
	case models.SpacesVM:
		if err := d.printExtraSpacesVMInfo(chainGenesis); err != nil {
			// not supposed to happen due to genesis pre validation
			return ids.Empty, ids.Empty, nil
		}
	}

	// we can safely ignore errors here as the subnets have already been generated
	subnetID, _ := ids.FromString(subnetIDStr)
	var blockchainID ids.ID
	for _, info := range clusterInfo.CustomChains {
		if info.VmId == chainVMID.String() {
			blockchainID, _ = ids.FromString(info.ChainId)
		}
	}
	return subnetID, blockchainID, nil
}

func (*LocalDeployer) printExtraSpacesVMInfo(chainGenesis []byte) error {
	var genesis spacesvmchain.Genesis
	if err := json.Unmarshal(chainGenesis, &genesis); err != nil {
		return fmt.Errorf("failed to unmarshall genesis: %w", err)
	}
	for _, alloc := range genesis.CustomAllocation {
		address := alloc.Address
		amount := alloc.Balance
		amountStr := fmt.Sprintf("%d", amount)
		if address == vm.PrefundedEwoqAddress {
			ux.Logger.PrintToUser("Funded address:   %s with %s - private key: %s", address, amountStr, vm.PrefundedEwoqPrivate)
		} else {
			ux.Logger.PrintToUser("Funded address:   %s with %s", address, amountStr)
		}
	}
	return nil
}

func (d *LocalDeployer) printExtraEvmInfo(chain string, chainGenesis []byte) error {
	var evmGenesis core.Genesis
	if err := json.Unmarshal(chainGenesis, &evmGenesis); err != nil {
		return fmt.Errorf("failed to unmarshall genesis: %w", err)
	}
	for address := range evmGenesis.Alloc {
		amount := evmGenesis.Alloc[address].Balance
		formattedAmount := new(big.Int).Div(amount, big.NewInt(params.Ether))
		if address == vm.PrefundedEwoqAddress {
			ux.Logger.PrintToUser("Funded address:   %s with %s (10^18) - private key: %s", address, formattedAmount.String(), vm.PrefundedEwoqPrivate)
		} else {
			ux.Logger.PrintToUser("Funded address:   %s with %s", address, formattedAmount.String())
		}
	}
	ux.Logger.PrintToUser("Network name:     %s", chain)
	ux.Logger.PrintToUser("Chain ID:         %s", evmGenesis.Config.ChainID)
	ux.Logger.PrintToUser("Currency Symbol:  %s", d.app.GetTokenName(chain))
	return nil
}

// SetupLocalEnv also does some heavy lifting:
// * sets up default snapshot if not installed
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads it and installs it (os - and archive dependent)
// * returns the location of the avalanchego path and plugin
func (d *LocalDeployer) SetupLocalEnv() (string, string, string, error) {
	err := d.setDefaultSnapshot(d.app.GetSnapshotsDir(), false)
	if err != nil {
		return "", "", "", fmt.Errorf("failed setting up snapshots: %w", err)
	}

	avagoVersion, avagoDir, err := d.setupLocalEnv()
	if err != nil {
		return "", "", "", fmt.Errorf("failed setting up local environment: %w", err)
	}

	pluginDir := filepath.Join(avagoDir, "plugins")
	avalancheGoBinPath := filepath.Join(avagoDir, "avalanchego")

	if err := os.MkdirAll(pluginDir, constants.DefaultPerms755); err != nil {
		return "", "", "", fmt.Errorf("could not create pluginDir %s", pluginDir)
	}

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return "", "", "", fmt.Errorf("evaluated pluginDir to be %s but it does not exist", pluginDir)
	}

	// TODO: we need some better version management here
	// * compare latest to local version
	// * decide if force update or give user choice
	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return "", "", "", fmt.Errorf(
			"evaluated avalancheGoBinPath to be %s but it does not exist", avalancheGoBinPath)
	}

	return avagoVersion, avalancheGoBinPath, pluginDir, nil
}

func (d *LocalDeployer) setupLocalEnv() (string, string, error) {
	return binutils.SetupAvalanchego(d.app, d.avagoVersion)
}

// WaitForHealthy polls continuously until the network is ready to be used
func (d *LocalDeployer) WaitForHealthy(
	ctx context.Context,
	cli client.Client,
	healthCheckInterval time.Duration,
) (*rpcpb.ClusterInfo, error) {
	cancel := make(chan struct{})
	defer close(cancel)
	go ux.PrintWait(cancel)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(healthCheckInterval):
			d.app.Log.Debug("polling for health...")
			resp, err := cli.Health(ctx)
			if err != nil {
				return nil, err
			}
			if resp.ClusterInfo == nil {
				d.app.Log.Debug("warning: ClusterInfo is nil. trying again...")
				continue
			}
			if !resp.ClusterInfo.Healthy {
				d.app.Log.Debug("network is not healthy. polling again...")
				continue
			}
			if !resp.ClusterInfo.CustomChainsHealthy {
				d.app.Log.Debug("network is up but custom VMs are not healthy. polling again...")
				continue
			}
			d.app.Log.Debug("network is up and custom VMs are up")
			return resp.ClusterInfo, nil
		}
	}
}

// GetEndpoints get a human readable list of endpoints from clusterinfo
func GetEndpoints(clusterInfo *rpcpb.ClusterInfo) []string {
	endpoints := []string{}
	for _, nodeInfo := range clusterInfo.NodeInfos {
		for blockchainID, chainInfo := range clusterInfo.CustomChains {
			endpoints = append(endpoints, fmt.Sprintf("Endpoint at node %s for blockchain %q with VM ID %q: %s/ext/bc/%s/rpc", nodeInfo.Name, blockchainID, chainInfo.VmId, nodeInfo.GetUri(), blockchainID))
		}
	}
	return endpoints
}

// return true if vm has already been deployed
func alreadyDeployed(chainVMID ids.ID, clusterInfo *rpcpb.ClusterInfo) bool {
	if clusterInfo != nil {
		for _, chainInfo := range clusterInfo.CustomChains {
			if chainInfo.VmId == chainVMID.String() {
				return true
			}
		}
	}
	return false
}

// get list of all needed plugins and install them
func (d *LocalDeployer) installPlugin(
	vmID ids.ID,
	vmBin string,
	pluginDir string,
) error {
	return d.binaryDownloader.InstallVM(vmID.String(), vmBin, pluginDir)
}

func getExpectedDefaultSnapshotSHA256Sum() (string, error) {
	resp, err := http.Get(constants.BootstrapSnapshotSHA256URL)
	if err != nil {
		return "", fmt.Errorf("failed downloading sha256 sums: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed downloading sha256 sums: unexpected http status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	sha256FileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed downloading sha256 sums: %w", err)
	}
	expectedSum, err := utils.SearchSHA256File(sha256FileBytes, constants.BootstrapSnapshotLocalPath)
	if err != nil {
		return "", fmt.Errorf("failed obtaining snapshot sha256 sum: %w", err)
	}
	return expectedSum, nil
}

// Initialize default snapshot with bootstrap snapshot archive
// If force flag is set to true, overwrite the default snapshot if it exists
func SetDefaultSnapshot(snapshotsDir string, force bool) error {
	bootstrapSnapshotArchivePath := filepath.Join(snapshotsDir, constants.BootstrapSnapshotArchiveName)
	// will download either if file not exists or if sha256 sum is not the same
	downloadSnapshot := false
	if _, err := os.Stat(bootstrapSnapshotArchivePath); os.IsNotExist(err) {
		downloadSnapshot = true
	} else {
		gotSum, err := utils.GetSHA256FromDisk(bootstrapSnapshotArchivePath)
		if err != nil {
			return err
		}
		expectedSum, err := getExpectedDefaultSnapshotSHA256Sum()
		if err != nil {
			ux.Logger.PrintToUser("Warning: failure verifying that the local snapshot is the latest one: %s", err)
		} else if gotSum != expectedSum {
			downloadSnapshot = true
		}
	}
	if downloadSnapshot {
		resp, err := http.Get(constants.BootstrapSnapshotURL)
		if err != nil {
			return fmt.Errorf("failed downloading bootstrap snapshot: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed downloading bootstrap snapshot: unexpected http status code: %d", resp.StatusCode)
		}
		defer resp.Body.Close()
		bootstrapSnapshotBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed downloading bootstrap snapshot: %w", err)
		}
		if err := os.WriteFile(bootstrapSnapshotArchivePath, bootstrapSnapshotBytes, WriteReadReadPerms); err != nil {
			return fmt.Errorf("failed writing down bootstrap snapshot: %w", err)
		}
	}
	defaultSnapshotPath := filepath.Join(snapshotsDir, "anr-snapshot-"+constants.DefaultSnapshotName)
	if force {
		if err := os.RemoveAll(defaultSnapshotPath); err != nil {
			return fmt.Errorf("failed removing default snapshot: %w", err)
		}
	}
	if _, err := os.Stat(defaultSnapshotPath); os.IsNotExist(err) {
		bootstrapSnapshotBytes, err := os.ReadFile(bootstrapSnapshotArchivePath)
		if err != nil {
			return fmt.Errorf("failed reading bootstrap snapshot: %w", err)
		}
		if err := binutils.InstallArchive("tar.gz", bootstrapSnapshotBytes, snapshotsDir); err != nil {
			return fmt.Errorf("failed installing bootstrap snapshot: %w", err)
		}
	}
	return nil
}

// start the network
func (d *LocalDeployer) startNetwork(
	ctx context.Context,
	cli client.Client,
	avagoVersion string,
	avalancheGoBinPath string,
	pluginDir string,
	runDir string,
) error {
	ux.Logger.PrintToUser("Starting network...")
	loadSnapshotOpts := []client.OpOption{
		client.WithExecPath(avalancheGoBinPath),
		client.WithRootDataDir(runDir),
		client.WithReassignPortsIfUsed(true),
	}

	// For avago version < AvalancheGoPluginDirFlagAdded, we use ANR default location for plugins dir,
	// for >= AvalancheGoPluginDirFlagAdded, we pass the param
	// TODO: review this once ANR includes proper avago version management
	if semver.Compare(avagoVersion, constants.AvalancheGoPluginDirFlagAdded) >= 0 {
		loadSnapshotOpts = append(loadSnapshotOpts, client.WithPluginDir(pluginDir))
	}

	// load global node configs if they exist
	configStr, err := d.app.Conf.LoadNodeConfig()
	if err != nil {
		return err
	}
	if configStr != "" {
		loadSnapshotOpts = append(loadSnapshotOpts, client.WithGlobalNodeConfig(configStr))
	}

	_, err = cli.LoadSnapshot(
		ctx,
		constants.DefaultSnapshotName,
		loadSnapshotOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to start network :%w", err)
	}
	return nil
}

// Returns an error if the server cannot be contacted. You may want to ignore this error.
func GetLocallyDeployedSubnets(app *application.Avalanche) (map[string]struct{}, error) {
	deployedNames := map[string]struct{}{}
	// if the server can not be contacted, or there is a problem with the query,
	// DO NOT FAIL, just print No for deployed status
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return nil, err
	}

	ctx := binutils.GetAsyncContext()
	resp, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}

	for _, chain := range resp.GetClusterInfo().CustomChains {
		deployedNames[chain.ChainName] = struct{}{}
	}

	return deployedNames, nil
}
