// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy your subnet to a network",
	Long: `Deploy your subnet to a network. Currently supports local network only. 
Starts an avalanche-network-runner in the background and deploys your subnet there.`,
	RunE: deploySubnet,
	Args: cobra.ExactArgs(1),
}

var (
	deployLocal bool
	force       bool
)

func getChainsInSubnet(subnetName string) ([]string, error) {
	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read baseDir :%w", err)
	}

	chains := []string{}

	for _, f := range files {
		if strings.Contains(f.Name(), sidecar_suffix) {
			// read in sidecar file
			path := filepath.Join(baseDir, f.Name())
			jsonBytes, err := os.ReadFile(path)
			if err != nil {
				return []string{}, fmt.Errorf("failed reading file %s: %w", path, err)
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return []string{}, fmt.Errorf("failed unmarshaling file %s: %w", path, err)
			}
			if sc.Subnet == subnetName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}

// deploySubnet is the cobra command run for deploying subnets
func deploySubnet(cmd *cobra.Command, args []string) error {
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(args[0])
	if err != nil {
		return fmt.Errorf("failed to getChainsInSubnet: %w", err)
	}

	if len(chains) == 0 {
		return errors.New("Invalid subnet " + args[0])
	}

	var network models.Network
	if deployLocal {
		network = models.Local
	} else {
		networkStr, err := prompts.CaptureList(
			"Choose a network to deploy on",
			[]string{models.Local.String(), models.Fuji.String(), models.Mainnet.String()},
		)
		if err != nil {
			return err
		}
		network = models.NetworkFromString(networkStr)
	}

	log.Info("Deploying %s to %s", chains, network.String())
	// TODO
	switch network {
	case models.Local:
		log.Debug("Deploy local")
		return deployToLocalNetwork(chains[0])
	default:
		return errors.New("Not implemented")
	}
}

// deployToLocalNetwork does the heavy lifting:
// * it checks the gRPC is running, if not, it starts it
// * kicks off the actual deployment
func deployToLocalNetwork(chain string) error {
	isRunning, err := IsServerProcessRunning()
	if err != nil {
		return fmt.Errorf("failed querying if server process is running: %w", err)
	}
	if !isRunning {
		log.Debug("gRPC server is not running")
		if err := startServerProcess(); err != nil {
			return fmt.Errorf("failed starting gRPC server process: %w", err)
		}
	}
	return doDeploy(chain)
}

// doDeploy the actual deployment to the network runner
func doDeploy(chain string) error {
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed getting current user: %w", err)
	}
	avagoDir, err := setupLocalEnv(usr.HomeDir)
	if err != nil {
		return fmt.Errorf("failed setting up local environment: %w", err)
	}

	log.Info("Avalanchego installation successful")

	pluginDir := filepath.Join(avagoDir, "plugins")
	avalancheGoBinPath := filepath.Join(avagoDir, "avalanchego")

	exists, err := storage.FolderExists(pluginDir)
	if !exists || err != nil {
		return fmt.Errorf("evaluated pluginDir to be %s but it does not exist.", pluginDir)
	}

	exists, err = storage.FileExists(avalancheGoBinPath)
	if !exists || err != nil {
		return fmt.Errorf("evaluated avalancheGoBinPath to be %s but it does not exist.", avalancheGoBinPath)
	}

	requestTimeout := 3 * time.Minute

	cli, err := client.New(client.Config{
		LogLevel:    gRPCClientLogLevel,
		Endpoint:    gRPCServerEndpoint,
		DialTimeout: gRPCDialTimeout,
	})
	if err != nil {
		return fmt.Errorf("failed creating gRPC Client: %w", err)
	}
	defer cli.Close()

	chain_genesis := filepath.Join(usr.HomeDir, BaseDirName, fmt.Sprintf("%s_genesis.json", chain))
	exists, err = storage.FileExists(chain_genesis)
	if !exists || err != nil {
		return fmt.Errorf("evaluated chain genesis file to be at %s but it does not seem to exist.", chain_genesis)
	}

	customVMs := map[string]string{
		chain: chain_genesis,
	}

	opts := []client.OpOption{
		// client.WithNumNodes(numNodes),
		client.WithPluginDir(pluginDir),
		client.WithCustomVMs(customVMs),
	}

	vmID, err := utils.VMID(chain)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", chain, err)
	}
	log.Debug("this VM will get ID: %s", vmID.String())

	if err := getVMBinary(vmID, pluginDir); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	// don't call since "start" is async
	// and the top-level context here "ctx" is passed
	// to all underlying function calls
	// just set the timeout to halt "Start" async ops
	// when the deadline is reached
	_ = cancel

	log.Info("VM ready. Trying to boot network...")
	info, err := cli.Start(
		ctx,
		avalancheGoBinPath,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("failed to start network :%s", err)
	}

	log.Debug(info.String())
	log.Info("Network has been booted. Wait until healthy. Please be patient, this will take some time...")

	endpoints, err := waitForHealthy(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to query network health: %s", err)
	}

	fmt.Println()
	log.Info("Network ready to use. Local network node endpoints:")
	for _, u := range endpoints {
		fmt.Println(u)
	}
	return nil
}

// setupLocalEnv also does some heavy lifting:
// * checks if avalanchego is installed in the local binary path
// * if not, it downloads it and installs it (os - and archive dependent)
// * returns the location of the avalanchego path
func setupLocalEnv(homeDir string) (string, error) {
	binDir := filepath.Join(homeDir, BaseDirName, avalancheCliBinDir)

	exists, latest, err := avagoExists(binDir)
	if err != nil {
		return "", fmt.Errorf("the avalanchego binary could not be found anywhere in %s", binDir)
	}
	if exists {
		log.Debug("local avalanchego found. skipping installation")
		return latest, nil
	}

	log.Info("Installing latest avalanchego version...")

	// TODO: Question if there is a less error prone (= simpler) way to install latest avalanchego
	// Maybe the binary package manager should also allow the actual avalanchego binary for download
	resp, err := http.Get(latestAvagoReleaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to download avalanchego binary: %w", err)
	}

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
	resp.Body.Close()

	log.Info("Latest avalanchego version is: %s", version)

	arch := runtime.GOARCH
	goos := runtime.GOOS
	avalanchegoURL := fmt.Sprintf("https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-linux-%s-%s.tar.gz", version, arch, version)
	if goos == "darwin" {
		avalanchegoURL = fmt.Sprintf("https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-macos-%s.zip", version, version)
	}
	// EXPERMENTAL WIN, no support
	if goos == "windows" {
		avalanchegoURL = fmt.Sprintf("https://github.com/ava-labs/avalanchego/releases/download/%s/avalanchego-win-%s-experimental.zip", version, version)
	}

	log.Debug("starting download from %s...", avalanchegoURL)

	resp, err = http.Get(avalanchegoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	archive, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	log.Debug("download successful. installing archive...")
	if err := installArchive(goos, archive, binDir); err != nil {
		return "", err
	}
	return filepath.Join(binDir, "avalanchego-"+version), nil
}

// waitForHealthy polls continuously until the network is ready to be used
func waitForHealthy(ctx context.Context, cli client.Client) ([]string, error) {
	cancel := make(chan struct{})
	go printWait(cancel)
	for {
		select {
		case <-ctx.Done():
			cancel <- struct{}{}
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			cancel <- struct{}{}
			log.Debug("polling for health...")
			go printWait(cancel)
			resp, err := cli.Health(ctx)
			// TODO sometimes it hangs here!
			if err != nil {
				if strings.Contains(err.Error(), "context deadline exceeded") {
					return nil, err
				}
				if log.GetDisplayLevel() > logging.Info {
					fmt.Println()
				}
				log.Debug("health call failed, retrying: %w", err)
				continue
			}
			if resp.ClusterInfo == nil {
				if log.GetDisplayLevel() > logging.Info {
					fmt.Println()
				}
				log.Debug("warning: ClusterInfo is nil. trying again...")
				continue
			}
			if len(resp.ClusterInfo.CustomVms) == 0 {
				if log.GetDisplayLevel() > logging.Info {
					fmt.Println()
				}
				log.Debug("network is up but custom VMs are not installed yet. polling again...")
				continue
			}
			endpoints := []string{}
			for _, nodeInfo := range resp.ClusterInfo.NodeInfos {
				for vmID, vmInfo := range resp.ClusterInfo.CustomVms {
					endpoints = append(endpoints, fmt.Sprintf("Endpoint at node %s for blockchain %q: %s/ext/bc/%s", nodeInfo.Name, vmID, nodeInfo.GetUri(), vmInfo.BlockchainId))
				}
			}
			log.Debug("all good!")
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
			if log.GetDisplayLevel() != logging.Info {
				fmt.Println()
			}
			return
		}
	}
}
