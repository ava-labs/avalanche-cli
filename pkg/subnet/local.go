// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/storage"
)

type SubnetDeployer struct {
	procChecker         binutils.ProcessChecker
	binChecker          binutils.BinaryChecker
	getClientFunc       getGRPCClientFunc
	binaryDownloader    binutils.PluginBinaryDownloader
	healthCheckInterval time.Duration
	log                 logging.Logger
	baseDir             string
}

func NewLocalSubnetDeployer(log logging.Logger, baseDir string) *SubnetDeployer {
	return &SubnetDeployer{
		procChecker:         binutils.NewProcessChecker(),
		binChecker:          binutils.NewBinaryChecker(),
		getClientFunc:       binutils.NewGRPCClient,
		binaryDownloader:    binutils.NewPluginBinaryDownloader(log),
		healthCheckInterval: 10 * time.Second,
		log:                 log,
		baseDir:             baseDir,
	}
}

type getGRPCClientFunc func() (client.Client, error)

// DeployToLocalNetwork does the heavy lifting:
// * it checks the gRPC is running, if not, it starts it
// * kicks off the actual deployment
func (d *SubnetDeployer) DeployToLocalNetwork(chain string, chain_genesis string) error {
	isRunning, err := d.procChecker.IsServerProcessRunning()
	if err != nil {
		return fmt.Errorf("failed querying if server process is running: %w", err)
	}
	if !isRunning {
		d.log.Debug("gRPC server is not running")
		if err := binutils.StartServerProcess(d.log); err != nil {
			return fmt.Errorf("failed starting gRPC server process: %w", err)
		}
	}
	return d.doDeploy(chain, chain_genesis)
}

// doDeploy the actual deployment to the network runner
func (d *SubnetDeployer) doDeploy(chain string, chain_genesis string) error {
	avagoDir, err := d.setupLocalEnv()
	if err != nil {
		return fmt.Errorf("failed setting up local environment: %w", err)
	}

	ux.Logger.PrintToUser("Avalanchego installation successful")

	pluginDir := filepath.Join(avagoDir, "plugins")
	avalancheGoBinPath := filepath.Join(avagoDir, "avalanchego")

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return fmt.Errorf("evaluated pluginDir to be %s but it does not exist.", pluginDir)
	}

	// TODO: we need some better version management here
	// * compare latest to local version
	// * decide if force update or give user choice
	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return fmt.Errorf("evaluated avalancheGoBinPath to be %s but it does not exist.", avalancheGoBinPath)
	}

	cli, err := d.getClientFunc()
	if err != nil {
		return fmt.Errorf("error creating gRPC Client: %s", err)
	}
	defer cli.Close()

	exists, err = storage.FileExists(chain_genesis)
	if !exists || err != nil {
		return fmt.Errorf(
			"evaluated chain genesis file to be at %s but it does not seem to exist.", chain_genesis)
	}

	customVMs := map[string]string{
		chain: chain_genesis,
	}

	opts := []client.OpOption{
		client.WithPluginDir(pluginDir),
		client.WithCustomVMs(customVMs),
		client.WithGlobalNodeConfig("{\"log-level\":\"debug\", \"log-display-level\":\"debug\"}"),
	}

	vmID, err := utils.VMID(chain)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	d.log.Debug("this VM will get ID: %s", vmID.String())

	if err := d.binaryDownloader.Download(vmID, pluginDir); err != nil {
		return err
	}

	ctx := binutils.GetAsyncContext()

	ux.Logger.PrintToUser("VM ready. Trying to boot network...")
	info, err := cli.Start(
		ctx,
		avalancheGoBinPath,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("failed to start network :%s", err)
	}

	d.log.Debug(info.String())
	ux.Logger.PrintToUser("Network has been booted. Wait until healthy. Please be patient, this will take some time...")

	endpoints, err := d.WaitForHealthy(ctx, cli, d.healthCheckInterval)
	if err != nil {
		return fmt.Errorf("failed to query network health: %s", err)
	}

	fmt.Println()
	ux.Logger.PrintToUser("Network ready to use. Local network node endpoints:")
	for _, u := range endpoints {
		ux.Logger.PrintToUser(u)
	}
	return nil
}

// setupLocalEnv also does some heavy lifting:
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads it and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func (d *SubnetDeployer) setupLocalEnv() (string, error) {
	binDir := filepath.Join(d.baseDir, constants.AvalancheCliBinDir)

	exists, latest, err := d.binChecker.ExistsWithLatestVersion(binDir)
	if err != nil {
		return "", fmt.Errorf("failed trying to locate avalanchego binary: %s", binDir)
	}
	if exists {
		d.log.Debug("local avalanchego found. skipping installation")
		return latest, nil
	}

	ux.Logger.PrintToUser("Installing latest avalanchego version...")

	version, err := getLatestAvagoVersion(constants.LatestAvagoReleaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to get latest avalanchego version: %s", err)
	}

	d.log.Info("Latest avalanchego version is: %s", version)

	arch := runtime.GOARCH
	goos := runtime.GOOS
	var avalanchegoURL string

	switch goos {
	case "linux":
		avalanchegoURL = fmt.Sprintf(
			"https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-linux-%s-%s.tar.gz",
			version,
			arch,
			version,
		)
	case "darwin":
		avalanchegoURL = fmt.Sprintf(
			"https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-macos-%s.zip",
			version,
			version,
		)
		// EXPERMENTAL WIN, no support
	case "windows":
		avalanchegoURL = fmt.Sprintf(
			"https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-win-%s-experimental.zip",
			version,
			version,
		)
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
	if err := binutils.InstallArchive(goos, archive, binDir); err != nil {
		return "", err
	}
	return filepath.Join(binDir, "avalanchego-"+version), nil
}

func getLatestAvagoVersion(releaseURL string) (string, error) {
	// TODO: Question if there is a less error prone (= simpler) way to install latest avalanchego
	// Maybe the binary package manager should also allow the actual avalanchego binary for download
	resp, err := http.Get(releaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to download avalanchego binary: %w", err)
	}
	defer resp.Body.Close()

	jsonBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to get latest avalanchego version: %w", err)
	}

	var jsonStr map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &jsonStr); err != nil {
		return "", fmt.Errorf("failed to unmarshal avalanchego json version string: %w", err)
	}

	version := jsonStr["tag_name"].(string)
	if version == "" || version[0] != 'v' {
		return "", fmt.Errorf("invalid version string: %s", version)
	}

	return version, nil
}

// WaitForHealthy polls continuously until the network is ready to be used
func (d *SubnetDeployer) WaitForHealthy(ctx context.Context, cli client.Client, healthCheckInterval time.Duration) ([]string, error) {
	cancel := make(chan struct{})
	go printWait(cancel)
	for {
		select {
		case <-ctx.Done():
			cancel <- struct{}{}
			return nil, ctx.Err()
		case <-time.After(healthCheckInterval):
			cancel <- struct{}{}
			d.log.Debug("polling for health...")
			go printWait(cancel)
			resp, err := cli.Health(ctx)
			if err != nil {
				return nil, fmt.Errorf("the health check failed to complete. The server might be down or have crashed, check the logs! %s", err)
			}
			if resp.ClusterInfo == nil {
				d.log.Debug("warning: ClusterInfo is nil. trying again...")
				continue
			}
			if len(resp.ClusterInfo.CustomVms) == 0 {
				d.log.Debug("network is up but custom VMs are not installed yet. polling again...")
				continue
			}
			if !resp.ClusterInfo.CustomVmsHealthy {
				d.log.Debug("network is up but custom VMs are not healthy. polling again...")
				continue
			}
			endpoints := []string{}
			for _, nodeInfo := range resp.ClusterInfo.NodeInfos {
				for vmID, vmInfo := range resp.ClusterInfo.CustomVms {
					endpoints = append(endpoints, fmt.Sprintf("Endpoint at node %s for blockchain %q: %s/ext/bc/%s", nodeInfo.Name, vmID, nodeInfo.GetUri(), vmInfo.BlockchainId))
				}
			}
			d.log.Debug("cluster is up, subnets deployed, VMs are installed!")
			close(cancel)
			return endpoints, nil
		}
	}
}

// printWait does some dot printing to entertain the user
func printWait(cancel chan struct{}) {
	for {
		select {
		case <-time.After(1 * time.Second):
			fmt.Print(".")
		case <-cancel:
			return
		}
	}
}
