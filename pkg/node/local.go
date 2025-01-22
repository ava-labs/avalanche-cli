// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	anrnetwork "github.com/ava-labs/avalanche-network-runner/network"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

func TrackSubnetWithLocalMachine(
	app *application.Avalanche,
	clusterName,
	blockchainName string,
	avalancheGoBinPath string,
) error {
	if ok, err := CheckClusterIsLocal(app, clusterName); err != nil || !ok {
		return fmt.Errorf("local node %q is not found", clusterName)
	}
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	network := models.GetNetworkFromCluster(clusterConfig)
	if network.ClusterName != "" {
		network = models.ConvertClusterToNetwork(network)
	}
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, network.Name())
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	blockchainID := sc.Networks[network.Name()].BlockchainID
	vmID, err := anrutils.VMID(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", blockchainName, err)
	}
	var vmBin string
	switch sc.VM {
	case models.SubnetEvm:
		_, vmBin, err = binutils.SetupSubnetEVM(app, sc.VMVersion)
		if err != nil {
			return fmt.Errorf("failed to install subnet-evm: %w", err)
		}
	case models.CustomVM:
		vmBin = binutils.SetupCustomBin(app, blockchainName)
	default:
		return fmt.Errorf("unknown vm: %s", sc.VM)
	}
	rootDir := app.GetLocalDir(clusterName)
	pluginPath := filepath.Join(rootDir, "node1", "plugins", vmID.String())
	if err := utils.FileCopy(vmBin, pluginPath); err != nil {
		return err
	}
	if err := os.Chmod(pluginPath, constants.DefaultPerms755); err != nil {
		return err
	}

	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}
	ctx, cancel := network.BootstrappingContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	networkInfo := sc.Networks[network.Name()]
	rpcEndpoints := []string{}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		ux.Logger.PrintToUser("Restarting node %s to track newly deployed network", nodeInfo.Name)
		if err := LocalNodeTrackSubnet(
			ctx,
			cli,
			app,
			rootDir,
			avalancheGoBinPath,
			blockchainName,
			blockchainID,
			subnetID,
			nodeInfo.Name,
		); err != nil {
			return err
		}
		if err := AddNodeInfoToSidecar(&sc, nodeInfo, network); err != nil {
			return fmt.Errorf("failed to update sidecar with new node info: %w", err)
		}
		rpcEndpoints = append(rpcEndpoints, models.GetRPCEndpoint(nodeInfo.Uri, networkInfo.BlockchainID.String()))
	}
	ux.Logger.PrintToUser("Waiting for blockchain %s to be bootstrapped", blockchainName)
	if err := WaitBootstrapped(ctx, cli, blockchainID.String()); err != nil {
		return fmt.Errorf("failure waiting for local cluster %s bootstrapping: %w", blockchainName, err)
	}
	for _, rpcURL := range rpcEndpoints {
		ux.Logger.PrintToUser("Waiting for rpc %s to be available", rpcURL)
		if err := evm.WaitForRPC(ctx, rpcURL); err != nil {
			return err
		}
	}
	if err := app.UpdateSidecar(&sc); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("%s successfully tracking %s", clusterName, blockchainName)
	return nil
}

func LocalNodeTrackSubnet(
	ctx context.Context,
	cli client.Client,
	app *application.Avalanche,
	rootDir string,
	avalancheGoBinPath string,
	blockchainName string,
	blockchainID ids.ID,
	subnetID ids.ID,
	nodeName string,
) error {
	if app.ChainConfigExists(blockchainName) {
		inputChainConfigPath := app.GetChainConfigPath(blockchainName)
		outputChainConfigPath := filepath.Join(rootDir, nodeName, "configs", "chains", blockchainID.String(), "config.json")
		ux.Logger.Info("Creating chain conf directory %s", filepath.Dir(outputChainConfigPath))
		if err := os.MkdirAll(filepath.Dir(outputChainConfigPath), 0o700); err != nil {
			return fmt.Errorf("could not create chain conf directory %s: %w", filepath.Dir(outputChainConfigPath), err)
		}
		ux.Logger.Info("Copying %s to %s", inputChainConfigPath, outputChainConfigPath)
		if err := utils.FileCopy(inputChainConfigPath, outputChainConfigPath); err != nil {
			return err
		}
	}

	opts := []client.OpOption{
		client.WithWhitelistedSubnets(subnetID.String()),
		client.WithRootDataDir(rootDir),
		client.WithExecPath(avalancheGoBinPath),
	}
	ux.Logger.Info("Using client options: %v", opts)
	ux.Logger.Info("Restarting node %s", nodeName)
	if _, err := cli.RestartNode(ctx, nodeName, opts...); err != nil {
		return err
	}

	return nil
}

func CheckClusterIsLocal(app *application.Avalanche, clusterName string) (bool, error) {
	clustersConfig, err := app.GetClustersConfig()
	if err != nil {
		return false, err
	}
	clusterConf, ok := clustersConfig.Clusters[clusterName]
	return ok && clusterConf.Local, nil
}

func StartLocalNode(
	app *application.Avalanche,
	clusterName string,
	avalanchegoBinaryPath string,
	numNodes uint32,
	nodeConfig map[string]interface{},
	anrSettings ANRSettings,
	avaGoVersionSetting AvalancheGoVersionSettings,
	network models.Network,
	networkFlags networkoptions.NetworkFlags,
	supportedNetworkOptions []networkoptions.NetworkOption,
) error {
	var err error

	// ensure data consistency
	localClusterExists := false
	if clusterExists, err := CheckClusterExists(app, clusterName); err != nil {
		return fmt.Errorf("error checking clusters info: %w", err)
	} else if clusterExists {
		if localClusterExists, err = CheckClusterIsLocal(app, clusterName); err != nil {
			return fmt.Errorf("error verifying if cluster is local: %w", err)
		} else if !localClusterExists {
			return fmt.Errorf("cluster %s is not a local one", clusterName)
		}
	}
	localDataExists := localClusterDataExists(app, clusterName)
	if (localClusterExists && !localDataExists) || (!localClusterExists && localDataExists) {
		ux.Logger.RedXToUser("Inconsistent state for cluster: Cleaning up")
		_ = DestroyLocalNode(app, clusterName)
		localClusterExists = false
		localDataExists = false
	}

	// check if this is existing cluster
	rootDir := app.GetLocalDir(clusterName)
	pluginDir := filepath.Join(rootDir, "node1", "plugins")
	// make sure rootDir exists
	if err := os.MkdirAll(rootDir, 0o700); err != nil {
		return fmt.Errorf("could not create root directory %s: %w", rootDir, err)
	}
	// make sure pluginDir exists
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		return fmt.Errorf("could not create plugin directory %s: %w", pluginDir, err)
	}

	ctx, cancel := utils.GetANRContext()
	defer cancel()

	// starts server
	avalancheGoVersion := "latest"
	if avalanchegoBinaryPath == "" {
		avalancheGoVersion, err = GetAvalancheGoVersion(app, avaGoVersionSetting)
		if err != nil {
			return err
		}
		_, avagoDir, err := binutils.SetupAvalanchego(app, avalancheGoVersion)
		if err != nil {
			return fmt.Errorf("failed installing Avalanche Go version %s: %w", avalancheGoVersion, err)
		}
		avalanchegoBinaryPath = filepath.Join(avagoDir, "avalanchego")
		ux.Logger.PrintToUser("Using AvalancheGo version: %s", avalancheGoVersion)
	}
	serverLogPath := filepath.Join(rootDir, "server.log")
	sd := subnet.NewLocalDeployer(app, avalancheGoVersion, avalanchegoBinaryPath, "", true)
	if err := sd.StartServer(
		constants.ServerRunFileLocalClusterPrefix,
		binutils.LocalClusterGRPCServerPort,
		binutils.LocalClusterGRPCGatewayPort,
		rootDir,
		serverLogPath,
	); err != nil {
		return err
	}
	avalancheGoBinPath, err := sd.SetupLocalEnv()
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("AvalancheGo path: %s\n", avalancheGoBinPath)
	cli, err := binutils.NewGRPCClientWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
	if err != nil {
		return err
	}
	alreadyBootstrapped, err := localnet.IsBootstrapped(ctx, cli)
	if err != nil {
		return err
	}
	if alreadyBootstrapped {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("A local cluster is already executing")
		ux.Logger.PrintToUser("please stop it by calling 'node local stop'")
		return nil
	}

	if nodeConfig == nil {
		nodeConfig = map[string]interface{}{}
	}
	nodeConfig[config.NetworkAllowPrivateIPsKey] = true
	nodeConfig[config.IndexEnabledKey] = false
	nodeConfig[config.IndexAllowIncompleteKey] = true

	nodeConfigBytes, err := json.Marshal(nodeConfig)
	if err != nil {
		return err
	}
	nodeConfigStr := string(nodeConfigBytes)
	if localClusterExists && localDataExists {
		ux.Logger.GreenCheckmarkToUser("Local cluster %s found. Booting up...", clusterName)
		loadSnapshotOpts := []client.OpOption{
			client.WithExecPath(avalancheGoBinPath),
			client.WithReassignPortsIfUsed(true),
			client.WithPluginDir(pluginDir),
			client.WithSnapshotPath(rootDir),
			client.WithGlobalNodeConfig(nodeConfigStr),
		}
		// load snapshot for existing network
		if _, err := cli.LoadSnapshot(
			ctx,
			clusterName,
			true, // in-place
			loadSnapshotOpts...,
		); err != nil {
			return fmt.Errorf("failed to load snapshot: %w", err)
		}
	} else {
		ux.Logger.GreenCheckmarkToUser("Local cluster %s not found. Creating...", clusterName)
		if network.Kind == models.Undefined {
			network, err = networkoptions.GetNetworkFromCmdLineFlags(
				app,
				"",
				networkFlags,
				false,
				true,
				supportedNetworkOptions,
				"",
			)
			if err != nil {
				return err
			}
		}
		network.ClusterName = clusterName

		if err := preLocalChecks(anrSettings, avaGoVersionSetting); err != nil {
			return err
		}

		switch {
		case network.Kind == models.Fuji:
			ux.Logger.PrintToUser(logging.Yellow.Wrap("Warning: Fuji Bootstrapping can take several minutes"))
		case network.Kind == models.Mainnet:
			ux.Logger.PrintToUser(logging.Yellow.Wrap("Warning: Mainnet Bootstrapping can take 6-24 hours"))
		case network.Kind == models.Local:
			clusterInfo, err := localnet.GetClusterInfo()
			if err != nil {
				return fmt.Errorf("failed to connect to local network: %w", err)
			}
			rootDataDir := clusterInfo.RootDataDir
			networkJSONPath := filepath.Join(rootDataDir, "network.json")
			bs, err := os.ReadFile(networkJSONPath)
			if err != nil {
				return fmt.Errorf("could not read local network config file %s: %w", networkJSONPath, err)
			}
			var networkJSON anrnetwork.Config
			if err := json.Unmarshal(bs, &networkJSON); err != nil {
				return err
			}
			for id, ip := range networkJSON.BeaconConfig {
				anrSettings.BootstrapIDs = append(anrSettings.BootstrapIDs, id.String())
				anrSettings.BootstrapIPs = append(anrSettings.BootstrapIPs, ip.String())
			}
			// prepare genesis and upgrade files for anr
			genesisFile, err := os.CreateTemp("", "local_network_genesis")
			if err != nil {
				return fmt.Errorf("could not create local network genesis file: %w", err)
			}
			if _, err := genesisFile.Write([]byte(networkJSON.Genesis)); err != nil {
				return fmt.Errorf("could not write local network genesis file: %w", err)
			}
			if err := genesisFile.Close(); err != nil {
				return fmt.Errorf("could not close local network genesis file: %w", err)
			}
			anrSettings.GenesisPath = genesisFile.Name()
			defer os.Remove(anrSettings.GenesisPath)
			upgradeFile, err := os.CreateTemp("", "local_network_upgrade")
			if err != nil {
				return fmt.Errorf("could not create local network upgrade file: %w", err)
			}
			if _, err := upgradeFile.Write([]byte(networkJSON.Upgrade)); err != nil {
				return fmt.Errorf("could not write local network upgrade file: %w", err)
			}
			anrSettings.UpgradePath = upgradeFile.Name()
			if err := upgradeFile.Close(); err != nil {
				return fmt.Errorf("could not close local network upgrade file: %w", err)
			}
			defer os.Remove(anrSettings.UpgradePath)
		}

		if anrSettings.StakingTLSKeyPath != "" && anrSettings.StakingCertKeyPath != "" && anrSettings.StakingSignerKeyPath != "" {
			if err := os.MkdirAll(filepath.Join(rootDir, "node1", "staking"), 0o700); err != nil {
				return fmt.Errorf("could not create root directory %s: %w", rootDir, err)
			}
			if err := utils.FileCopy(anrSettings.StakingTLSKeyPath, filepath.Join(rootDir, "node1", "staking", "staker.key")); err != nil {
				return err
			}
			if err := utils.FileCopy(anrSettings.StakingCertKeyPath, filepath.Join(rootDir, "node1", "staking", "staker.crt")); err != nil {
				return err
			}
			if err := utils.FileCopy(anrSettings.StakingSignerKeyPath, filepath.Join(rootDir, "node1", "staking", "signer.key")); err != nil {
				return err
			}
		}

		anrOpts := []client.OpOption{
			client.WithNumNodes(numNodes),
			client.WithNetworkID(network.ID),
			client.WithExecPath(avalancheGoBinPath),
			client.WithRootDataDir(rootDir),
			client.WithReassignPortsIfUsed(true),
			client.WithPluginDir(pluginDir),
			client.WithFreshStakingIds(true),
			client.WithZeroIP(false),
			client.WithGlobalNodeConfig(nodeConfigStr),
		}
		if anrSettings.GenesisPath != "" && utils.FileExists(anrSettings.GenesisPath) {
			anrOpts = append(anrOpts, client.WithGenesisPath(anrSettings.GenesisPath))
		}
		if anrSettings.UpgradePath != "" && utils.FileExists(anrSettings.UpgradePath) {
			anrOpts = append(anrOpts, client.WithUpgradePath(anrSettings.UpgradePath))
		}
		if anrSettings.BootstrapIDs != nil {
			anrOpts = append(anrOpts, client.WithBootstrapNodeIDs(anrSettings.BootstrapIDs))
		}
		if anrSettings.BootstrapIPs != nil {
			anrOpts = append(anrOpts, client.WithBootstrapNodeIPPortPairs(anrSettings.BootstrapIPs))
		}

		ctx, cancel = network.BootstrappingContext()
		defer cancel()

		ux.Logger.PrintToUser("Starting local avalanchego node using root: %s ...", rootDir)
		spinSession := ux.NewUserSpinner()
		spinner := spinSession.SpinToUser("Booting Network. Wait until healthy...")
		// preseed nodes data from public archive. ignore errors
		nodeNames := []string{}
		for i := 1; i <= int(numNodes); i++ {
			nodeNames = append(nodeNames, fmt.Sprintf("node%d", i))
		}
		err := DownloadPublicArchive(network, rootDir, nodeNames)
		ux.Logger.Info("seeding public archive data finished with error: %v. Ignored if any", err)

		if _, err := cli.Start(ctx, avalancheGoBinPath, anrOpts...); err != nil {
			ux.SpinFailWithError(spinner, "", err)
			_ = DestroyLocalNode(app, clusterName)
			return fmt.Errorf("failed to start local avalanchego: %w", err)
		}
		ux.SpinComplete(spinner)
		spinSession.Stop()
		// save cluster config for the new local cluster
		if err := addLocalClusterConfig(app, network); err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("Waiting for P-Chain to be bootstrapped")
	if err := WaitBootstrapped(ctx, cli, "P"); err != nil {
		return fmt.Errorf("failure waiting for local cluster P-Chain bootstrapping: %w", err)
	}

	ux.Logger.GreenCheckmarkToUser("Avalanchego started and ready to use from %s", rootDir)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Node logs directory: %s/node1/logs", rootDir)
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Network ready to use.")
	ux.Logger.PrintToUser("")

	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		ux.Logger.PrintToUser("URI: %s", nodeInfo.Uri)
		ux.Logger.PrintToUser("NodeID: %s", nodeInfo.Id)
		ux.Logger.PrintToUser("")
	}

	return nil
}

// add additional validator to local node
func UpsizeLocalNode(
	app *application.Avalanche,
	network models.Network,
	blockchainName string,
	blockchainID ids.ID,
	subnetID ids.ID,
	avalancheGoBinPath string,
	nodeConfig map[string]interface{},
	anrSettings ANRSettings,
) (
	string, // added nodeName
	error,
) {
	clusterName, err := GetRunnningLocalNodeClusterName(app)
	if err != nil {
		return "", err
	}
	rootDir := app.GetLocalDir(clusterName)
	pluginDir := filepath.Join(rootDir, "node1", "plugins")

	if nodeConfig == nil {
		nodeConfig = map[string]interface{}{}
	}
	nodeConfig[config.NetworkAllowPrivateIPsKey] = true
	if network.Kind == models.Fuji {
		nodeConfig[config.IndexEnabledKey] = false // disable index for Fuji
	}
	nodeConfigBytes, err := json.Marshal(nodeConfig)
	if err != nil {
		return "", err
	}
	nodeConfigStr := string(nodeConfigBytes)

	// we will remove this code soon, so it can be not DRY
	if network.Kind == models.Local {
		clusterInfo, err := localnet.GetClusterInfo()
		if err != nil {
			return "", fmt.Errorf("failed to connect to local network: %w", err)
		}
		rootDataDir := clusterInfo.RootDataDir
		networkJSONPath := filepath.Join(rootDataDir, "network.json")
		bs, err := os.ReadFile(networkJSONPath)
		if err != nil {
			return "", fmt.Errorf("could not read local network config file %s: %w", networkJSONPath, err)
		}
		var networkJSON anrnetwork.Config
		if err := json.Unmarshal(bs, &networkJSON); err != nil {
			return "", err
		}
		for id, ip := range networkJSON.BeaconConfig {
			anrSettings.BootstrapIDs = append(anrSettings.BootstrapIDs, id.String())
			anrSettings.BootstrapIPs = append(anrSettings.BootstrapIPs, ip.String())
		}
		// prepare genesis and upgrade files for anr
		genesisFile, err := os.CreateTemp("", "local_network_genesis")
		if err != nil {
			return "", fmt.Errorf("could not create local network genesis file: %w", err)
		}
		if _, err := genesisFile.Write([]byte(networkJSON.Genesis)); err != nil {
			return "", fmt.Errorf("could not write local network genesis file: %w", err)
		}
		if err := genesisFile.Close(); err != nil {
			return "", fmt.Errorf("could not close local network genesis file: %w", err)
		}
		anrSettings.GenesisPath = genesisFile.Name()
		defer os.Remove(anrSettings.GenesisPath)
		upgradeFile, err := os.CreateTemp("", "local_network_upgrade")
		if err != nil {
			return "", fmt.Errorf("could not create local network upgrade file: %w", err)
		}
		if _, err := upgradeFile.Write([]byte(networkJSON.Upgrade)); err != nil {
			return "", fmt.Errorf("could not write local network upgrade file: %w", err)
		}
		anrSettings.UpgradePath = upgradeFile.Name()
		if err := upgradeFile.Close(); err != nil {
			return "", fmt.Errorf("could not close local network upgrade file: %w", err)
		}
		defer os.Remove(anrSettings.UpgradePath)
	}
	// end of code to be removed
	anrOpts := []client.OpOption{
		client.WithNetworkID(network.ID),
		client.WithExecPath(avalancheGoBinPath),
		client.WithRootDataDir(rootDir),
		client.WithWhitelistedSubnets(subnetID.String()),
		client.WithReassignPortsIfUsed(true),
		client.WithPluginDir(pluginDir),
		client.WithFreshStakingIds(true),
		client.WithZeroIP(false),
		client.WithGlobalNodeConfig(nodeConfigStr),
	}
	if anrSettings.GenesisPath != "" && utils.FileExists(anrSettings.GenesisPath) {
		anrOpts = append(anrOpts, client.WithGenesisPath(anrSettings.GenesisPath))
	}
	if anrSettings.UpgradePath != "" && utils.FileExists(anrSettings.UpgradePath) {
		anrOpts = append(anrOpts, client.WithUpgradePath(anrSettings.UpgradePath))
	}
	if anrSettings.BootstrapIDs != nil {
		anrOpts = append(anrOpts, client.WithBootstrapNodeIDs(anrSettings.BootstrapIDs))
	}
	if anrSettings.BootstrapIPs != nil {
		anrOpts = append(anrOpts, client.WithBootstrapNodeIPPortPairs(anrSettings.BootstrapIPs))
	}

	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return "", err
	}

	ctx, cancel := network.BootstrappingContext()
	defer cancel()

	newNodeName, err := GetNextNodeName()
	if err != nil {
		return "", fmt.Errorf("failed to generate a new node name: %w", err)
	}

	spinSession := ux.NewUserSpinner()
	spinner := spinSession.SpinToUser("Creating new node with name %s on local machine", newNodeName)
	err = DownloadPublicArchive(network, rootDir, []string{newNodeName})
	ux.Logger.Info("seeding public archive data finished with error: %v. Ignored if any", err)
	// add new local node
	if _, err := cli.AddNode(ctx, newNodeName, avalancheGoBinPath, anrOpts...); err != nil {
		ux.SpinFailWithError(spinner, "", err)
		return newNodeName, fmt.Errorf("failed to add local validator: %w", err)
	}
	ux.Logger.Info("Waiting for node: %s to be bootstrapping P-Chain", newNodeName)
	if err := WaitBootstrapped(ctx, cli, "P"); err != nil {
		return newNodeName, fmt.Errorf("failure waiting for local cluster P-Chain bootstrapping: %w", err)
	}
	ux.Logger.Info("Waiting for node: %s to be healthy", newNodeName)
	_, err = subnet.WaitForHealthy(ctx, cli)
	if err != nil {
		return newNodeName, fmt.Errorf("failed waiting for node %s to be healthy: %w", newNodeName, err)
	}
	ux.SpinComplete(spinner)
	spinner = spinSession.SpinToUser("Tracking blockchain %s", blockchainName)
	time.Sleep(10 * time.Second) // delay before restarting new node
	if err := LocalNodeTrackSubnet(ctx,
		cli,
		app,
		rootDir,
		avalancheGoBinPath,
		blockchainName,
		blockchainID,
		subnetID,
		newNodeName); err != nil {
		ux.SpinFailWithError(spinner, "", err)
		return newNodeName, fmt.Errorf("failed to track blockchain: %w", err)
	}
	// wait until cluster is healthy
	ux.Logger.Info("Waiting for node: %s to be bootstrapping %s", newNodeName, blockchainName)
	if err := WaitBootstrapped(ctx, cli, blockchainID.String()); err != nil {
		return newNodeName, fmt.Errorf("failure waiting for local cluster blockchain bootstrapping: %w", err)
	}
	spinner = spinSession.SpinToUser("Waiting for blockchain to be healthy")
	clusterInfo, err := subnet.WaitForHealthy(ctx, cli)
	if err != nil {
		return newNodeName, fmt.Errorf("failed waiting for blockchain to become healthy: %w", err)
	}
	ux.SpinComplete(spinner)
	spinSession.Stop()

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Node logs directory: %s/%s/logs", rootDir, newNodeName)
	ux.Logger.PrintToUser("")

	nodeInfo := clusterInfo.NodeInfos[newNodeName]
	ux.Logger.PrintToUser("Node name: %s ", newNodeName)
	ux.Logger.PrintToUser("URI: %s", nodeInfo.Uri)
	ux.Logger.PrintToUser("Node-ID: %s", nodeInfo.Id)
	ux.Logger.PrintToUser("")
	return newNodeName, nil
}

func localClusterDataExists(app *application.Avalanche, clusterName string) bool {
	rootDir := app.GetLocalDir(clusterName)
	return utils.FileExists(filepath.Join(rootDir, "state.json"))
}

// stub for now
func preLocalChecks(
	anrSettings ANRSettings,
	avaGoVersionSettings AvalancheGoVersionSettings,
) error {
	// expand passed paths
	if anrSettings.GenesisPath != "" {
		anrSettings.GenesisPath = utils.ExpandHome(anrSettings.GenesisPath)
	}
	if anrSettings.UpgradePath != "" {
		anrSettings.UpgradePath = utils.ExpandHome(anrSettings.UpgradePath)
	}
	// checks
	if avaGoVersionSettings.UseCustomAvalanchegoVersion != "" && (avaGoVersionSettings.UseLatestAvalanchegoReleaseVersion || avaGoVersionSettings.UseLatestAvalanchegoPreReleaseVersion) {
		return fmt.Errorf("specify either --custom-avalanchego-version or --latest-avalanchego-version")
	}
	if len(anrSettings.BootstrapIDs) != len(anrSettings.BootstrapIPs) {
		return fmt.Errorf("number of bootstrap IDs and bootstrap IP:port pairs must be equal")
	}
	if anrSettings.GenesisPath != "" && !utils.FileExists(anrSettings.GenesisPath) {
		return fmt.Errorf("genesis file %s does not exist", anrSettings.GenesisPath)
	}
	if anrSettings.UpgradePath != "" && !utils.FileExists(anrSettings.UpgradePath) {
		return fmt.Errorf("upgrade file %s does not exist", anrSettings.UpgradePath)
	}
	return nil
}

func addLocalClusterConfig(app *application.Avalanche, network models.Network) error {
	clusterName := network.ClusterName
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	clusterConfig.Local = true
	clusterConfig.Network = network
	clustersConfig.Clusters[clusterName] = clusterConfig
	return app.WriteClustersConfigFile(&clustersConfig)
}

func DestroyLocalNode(app *application.Avalanche, clusterName string) error {
	_ = StopLocalNode(app)

	rootDir := app.GetLocalDir(clusterName)
	if err := os.RemoveAll(rootDir); err != nil {
		return err
	}

	if ok, err := CheckClusterIsLocal(app, clusterName); err != nil || !ok {
		return fmt.Errorf("local cluster %q not found", clusterName)
	}

	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	delete(clustersConfig.Clusters, clusterName)
	if err := app.WriteClustersConfigFile(&clustersConfig); err != nil {
		return err
	}

	ux.Logger.GreenCheckmarkToUser("Local node %s cleaned up.", clusterName)
	return nil
}

func StopLocalNode(app *application.Avalanche) error {
	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	bootstrapped, err := localnet.IsBootstrapped(ctx, cli)
	if err != nil {
		return err
	}
	if bootstrapped {
		if _, err = cli.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop avalanchego: %w", err)
		}
	}
	if err := binutils.KillgRPCServerProcess(
		app,
		binutils.LocalClusterGRPCServerEndpoint,
		constants.ServerRunFileLocalClusterPrefix,
	); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("avalanchego stopped")
	return nil
}

func listLocalClusters(app *application.Avalanche, clusterNamesToInclude []string) (map[string]string, error) {
	localClusters := map[string]string{} // map[clusterName]rootDir
	clustersConfig, err := app.GetClustersConfig()
	if err != nil {
		return localClusters, err
	}
	for clusterName := range clustersConfig.Clusters {
		if len(clusterNamesToInclude) == 0 || slices.Contains(clusterNamesToInclude, clusterName) {
			if ok, err := CheckClusterIsLocal(app, clusterName); err == nil && ok {
				localClusters[clusterName] = app.GetLocalDir(clusterName)
			}
		}
	}
	return localClusters, nil
}

// ConnectedToLocalNetwork returns true if a local cluster is running
// and it is connected to a local network.
// It also returns the name of the cluster that is connected to the local network
func ConnectedToLocalNetwork(app *application.Avalanche) (bool, string, error) {
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	currentlyRunningRootDir := ""
	cli, _ := binutils.NewGRPCClientWithEndpoint( // ignore error as ANR might be not running
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if cli != nil {
		status, _ := cli.Status(ctx) // ignore error as ANR might be not running
		if status != nil && status.ClusterInfo != nil {
			if status.ClusterInfo.RootDataDir != "" {
				currentlyRunningRootDir = status.ClusterInfo.RootDataDir
			}
		}
	}
	if currentlyRunningRootDir == "" {
		return false, "", nil
	}
	localClusters, err := listLocalClusters(app, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to list local clusters: %w", err)
	}
	for clusterName, rootDir := range localClusters {
		clusterConf, err := app.GetClusterConfig(clusterName)
		if err != nil {
			return false, "", fmt.Errorf("failed to get cluster config: %w", err)
		}
		network := models.ConvertClusterToNetwork(clusterConf.Network)
		if rootDir == currentlyRunningRootDir && network.Kind == models.Local {
			return true, clusterName, nil
		}
	}
	return false, "", nil
}

func DestroyLocalNetworkConnectedCluster(app *application.Avalanche) error {
	isLocal, clusterName, err := ConnectedToLocalNetwork(app)
	if err != nil {
		return err
	}
	if isLocal {
		_ = DestroyLocalNode(app, clusterName)
	}
	return nil
}

func StopLocalNetworkConnectedCluster(app *application.Avalanche) error {
	isLocal, _, err := ConnectedToLocalNetwork(app)
	if err != nil {
		return err
	}
	if isLocal {
		return StopLocalNode(app)
	}
	return nil
}

func LocalStatus(app *application.Avalanche, clusterName string, blockchainName string) error {
	clustersToList := make([]string, 0)
	if clusterName != "" {
		if ok, err := CheckClusterIsLocal(app, clusterName); err != nil || !ok {
			return fmt.Errorf("local cluster %q not found", clusterName)
		}
		clustersToList = append(clustersToList, clusterName)
	}

	// get currently running local cluster
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	currentlyRunningRootDir := ""
	isHealthy := false
	cli, _ := binutils.NewGRPCClientWithEndpoint( // ignore error as ANR might be not running
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	runningAvagoURIs := []string{}
	if cli != nil {
		status, _ := cli.Status(ctx) // ignore error as ANR might be not running
		if status != nil && status.ClusterInfo != nil {
			if status.ClusterInfo.RootDataDir != "" {
				currentlyRunningRootDir = status.ClusterInfo.RootDataDir
			}
			isHealthy = status.ClusterInfo.Healthy
			// get list of the nodes
			for _, nodeInfo := range status.ClusterInfo.NodeInfos {
				runningAvagoURIs = append(runningAvagoURIs, nodeInfo.Uri)
			}
		}
	}
	localClusters, err := listLocalClusters(app, clustersToList)
	if err != nil {
		return fmt.Errorf("failed to list local clusters: %w", err)
	}
	if clusterName != "" {
		ux.Logger.PrintToUser("%s %s", logging.Blue.Wrap("Local cluster:"), logging.Green.Wrap(clusterName))
	} else {
		ux.Logger.PrintToUser(logging.Blue.Wrap("Local clusters:"))
	}
	for clusterName, rootDir := range localClusters {
		currenlyRunning := ""
		healthStatus := ""
		avagoURIOuput := ""

		clusterConf, err := app.GetClusterConfig(clusterName)
		if err != nil {
			return fmt.Errorf("failed to get cluster config: %w", err)
		}
		network := models.ConvertClusterToNetwork(clusterConf.Network)
		networkKind := fmt.Sprintf(" [%s]", logging.Orange.Wrap(network.Name()))

		// load sidecar and cluster config for the cluster  if blockchainName is not empty
		blockchainID := ids.Empty
		if blockchainName != "" {
			sc, err := app.LoadSidecar(blockchainName)
			if err != nil {
				return err
			}
			blockchainID = sc.Networks[network.Name()].BlockchainID
		}
		if rootDir == currentlyRunningRootDir {
			currenlyRunning = fmt.Sprintf(" [%s]", logging.Blue.Wrap("Running"))
			if isHealthy {
				healthStatus = fmt.Sprintf(" [%s]", logging.Green.Wrap("Healthy"))
			} else {
				healthStatus = fmt.Sprintf(" [%s]", logging.Red.Wrap("Unhealthy"))
			}
			for _, avagoURI := range runningAvagoURIs {
				nodeID, nodePOP, isBoot, err := GetInfo(avagoURI, blockchainID.String())
				if err != nil {
					ux.Logger.RedXToUser("failed to get node  %s info: %v", avagoURI, err)
					continue
				}
				nodePOPPubKey := "0x" + hex.EncodeToString(nodePOP.PublicKey[:])
				nodePOPProof := "0x" + hex.EncodeToString(nodePOP.ProofOfPossession[:])

				isBootStr := "Primary:" + logging.Red.Wrap("Not Bootstrapped")
				if isBoot {
					isBootStr = "Primary:" + logging.Green.Wrap("Bootstrapped")
				}

				blockchainStatus := ""
				if blockchainID != ids.Empty {
					blockchainStatus, _ = GetBlockchainStatus(avagoURI, blockchainID.String()) // silence errors
				}

				avagoURIOuput += fmt.Sprintf("   - %s [%s] [%s]\n     publicKey: %s \n     proofOfPossession: %s \n",
					logging.LightBlue.Wrap(avagoURI),
					nodeID,
					strings.TrimRight(strings.Join([]string{isBootStr, "L1:" + logging.Orange.Wrap(blockchainStatus)}, " "), " "),
					nodePOPPubKey,
					nodePOPProof,
				)
			}
		} else {
			currenlyRunning = fmt.Sprintf(" [%s]", logging.Black.Wrap("Stopped"))
		}
		ux.Logger.PrintToUser("- %s: %s %s %s %s", clusterName, rootDir, networkKind, currenlyRunning, healthStatus)
		ux.Logger.PrintToUser(avagoURIOuput)
	}

	return nil
}

func GetInfo(uri string, blockchainID string) (
	ids.NodeID, // nodeID
	*signer.ProofOfPossession, // nodePOP
	bool, // isBootstrapped
	error, // error
) {
	client := info.NewClient(uri)
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	nodeID, nodePOP, err := client.GetNodeID(ctx)
	if err != nil {
		return ids.EmptyNodeID, &signer.ProofOfPossession{}, false, err
	}
	isBootstrapped, err := client.IsBootstrapped(ctx, blockchainID)
	if err != nil {
		return nodeID, nodePOP, isBootstrapped, err
	}
	return nodeID, nodePOP, isBootstrapped, err
}

func GetBlockchainStatus(uri string, blockchainID string) (
	string, // status
	error, // error
) {
	client := platformvm.NewClient(uri)
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	status, err := client.GetBlockchainStatus(ctx, blockchainID)
	if err != nil {
		return "", err
	}
	if status.String() == "" {
		return "Not Syncing", nil
	}
	return status.String(), nil
}

func WaitBootstrapped(ctx context.Context, cli client.Client, blockchainID string) error {
	blockchainBootstrapCheckFrequency := time.Second
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		for {
			infoClient := info.NewClient(nodeInfo.GetUri())
			boostrapped, err := infoClient.IsBootstrapped(ctx, blockchainID)
			if err != nil && !strings.Contains(err.Error(), "there is no chain with alias/ID") {
				return err
			}
			if boostrapped {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(blockchainBootstrapCheckFrequency):
			}
		}
	}
	return err
}
