// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"os"
	"path/filepath"
)

func TrackSubnetWithLocalMachine(app *application.Avalanche, clusterName, blockchainName string) error {
	if ok, err := checkClusterIsLocal(app, clusterName); err != nil || !ok {
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
	network := clusterConfig.Network
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
	if app.ChainConfigExists(blockchainName) {
		inputChainConfigPath := app.GetChainConfigPath(blockchainName)
		outputChainConfigPath := filepath.Join(rootDir, "node1", "configs", "chains", blockchainID.String(), "config.json")
		if err := os.MkdirAll(filepath.Dir(outputChainConfigPath), 0o700); err != nil {
			return fmt.Errorf("could not create chain conf directory %s: %w", filepath.Dir(outputChainConfigPath), err)
		}
		if err := utils.FileCopy(inputChainConfigPath, outputChainConfigPath); err != nil {
			return err
		}
	}

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
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	publicEndpoints := []string{}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		if _, err := cli.RestartNode(ctx, nodeInfo.Name, client.WithWhitelistedSubnets(subnetID.String())); err != nil {
			return err
		}
		publicEndpoints = append(publicEndpoints, nodeInfo.Uri)
	}
	networkInfo := sc.Networks[network.Name()]
	rpcEndpoints := set.Of(networkInfo.RPCEndpoints...)
	wsEndpoints := set.Of(networkInfo.WSEndpoints...)
	for _, publicEndpoint := range publicEndpoints {
		rpcEndpoints.Add(getRPCEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
		wsEndpoints.Add(getWSEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
	}
	networkInfo.RPCEndpoints = rpcEndpoints.List()
	networkInfo.WSEndpoints = wsEndpoints.List()
	sc.Networks[clusterConfig.Network.Name()] = networkInfo
	if err := app.UpdateSidecar(&sc); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("%s successfully tracking %s", clusterName, blockchainName)
	return nil
}

func checkClusterIsLocal(app *application.Avalanche, clusterName string) (bool, error) {
	clustersConfig, err := app.GetClustersConfig()
	if err != nil {
		return false, err
	}
	clusterConf, ok := clustersConfig.Clusters[clusterName]
	return ok && clusterConf.Local, nil
}

func StartLocalNode(app *application.Avalanche, clusterName string, useEtnaDevnet bool, avalanchegoBinaryPath string, anrSettings ANRSettings, avaGoVersionSetting AvalancheGoVersionSettings, globalNetworkFlags networkoptions.NetworkFlags, createSupportedNetworkOptions []networkoptions.NetworkOption) error {
	network := models.UndefinedNetwork
	var err error

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
		} else {
			ux.Logger.PrintToUser("Using AvalancheGo version: %s", avalancheGoVersion)
		}
	}
	serverLogPath := filepath.Join(rootDir, "server.log")
	sd := subnet.NewLocalDeployer(app, avalancheGoVersion, avalanchegoBinaryPath, "")
	if err := sd.StartServer(
		constants.ServerRunFileLocalClusterPrefix,
		binutils.LocalClusterGRPCServerPort,
		binutils.LocalClusterGRPCGatewayPort,
		rootDir,
		serverLogPath,
	); err != nil {
		return err
	}
	_, avalancheGoBinPath, err := sd.SetupLocalEnv()
	if err != nil {
		return err
	}
	cli, err := binutils.NewGRPCClientWithEndpoint(binutils.LocalClusterGRPCServerEndpoint)
	if err != nil {
		return err
	}
	alreadyBootstrapped, err := localnet.CheckNetworkIsAlreadyBootstrapped(ctx, cli)
	if err != nil {
		return err
	}
	if alreadyBootstrapped {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("A local cluster is already executing")
		ux.Logger.PrintToUser("please stop it by calling 'node local stop'")
		return nil
	}

	if localClusterDataExists(app, clusterName) {
		ux.Logger.GreenCheckmarkToUser("Local cluster %s found. Booting up...", clusterName)
		loadSnapshotOpts := []client.OpOption{
			client.WithReassignPortsIfUsed(true),
			client.WithPluginDir(pluginDir),
			client.WithSnapshotPath(rootDir),
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
		if useEtnaDevnet {
			network = models.NewNetwork(
				models.Devnet,
				constants.EtnaDevnetNetworkID,
				constants.EtnaDevnetEndpoint,
				clusterName,
			)
		} else {
			network, err = networkoptions.GetNetworkFromCmdLineFlags(
				app,
				"",
				globalNetworkFlags,
				false,
				true,
				createSupportedNetworkOptions,
				"",
			)
			if err != nil {
				return err
			}
		}
		if err := preLocalChecks(app, clusterName, anrSettings, avaGoVersionSetting, avalanchegoBinaryPath, useEtnaDevnet, globalNetworkFlags); err != nil {
			return err
		}
		if useEtnaDevnet {
			anrSettings.BootstrapIDs = constants.EtnaDevnetBootstrapNodeIDs
			anrSettings.BootstrapIPs = constants.EtnaDevnetBootstrapIPs
			// prepare genesis and upgrade files for anr
			genesisFile, err := os.CreateTemp("", "etna_devnet_genesis")
			if err != nil {
				return fmt.Errorf("could not create save Etna Devnet genesis file: %w", err)
			}
			if _, err := genesisFile.Write(constants.EtnaDevnetGenesisData); err != nil {
				return fmt.Errorf("could not write Etna Devnet genesis data: %w", err)
			}
			if err := genesisFile.Close(); err != nil {
				return fmt.Errorf("could not close Etna Devnet genesis file: %w", err)
			}
			anrSettings.GenesisPath = genesisFile.Name()
			defer os.Remove(anrSettings.GenesisPath)

			upgradeFile, err := os.CreateTemp("", "etna_devnet_upgrade")
			if err != nil {
				return fmt.Errorf("could not create save Etna Devnet upgrade file: %w", err)
			}
			if _, err := upgradeFile.Write(constants.EtnaDevnetUpgradeData); err != nil {
				return fmt.Errorf("could not write Etna Devnet upgrade data: %w", err)
			}
			anrSettings.UpgradePath = upgradeFile.Name()
			if err := upgradeFile.Close(); err != nil {
				return fmt.Errorf("could not close Etna Devnet upgrade file: %w", err)
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
			client.WithNumNodes(1),
			client.WithNetworkID(network.ID),
			client.WithExecPath(avalancheGoBinPath),
			client.WithRootDataDir(rootDir),
			client.WithReassignPortsIfUsed(true),
			client.WithPluginDir(pluginDir),
			client.WithFreshStakingIds(true),
			client.WithZeroIP(false),
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

		ux.Logger.PrintToUser("Starting local avalanchego node using root: %s ...", rootDir)
		spinSession := ux.NewUserSpinner()
		spinner := spinSession.SpinToUser("Booting Network. Wait until healthy...")
		if _, err := cli.Start(ctx, avalancheGoBinPath, anrOpts...); err != nil {
			ux.SpinFailWithError(spinner, "", err)
			DestroyLocalNode(app, clusterName)
			return fmt.Errorf("failed to start local avalanchego: %w", err)
		}
		ux.SpinComplete(spinner)
		spinSession.Stop()
		// save cluster config for the new local cluster
		if err := addLocalClusterConfig(app, network); err != nil {
			return err
		}
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

func localClusterDataExists(app *application.Avalanche, clusterName string) bool {
	rootDir := app.GetLocalDir(clusterName)
	return utils.FileExists(filepath.Join(rootDir, "state.json"))
}

// stub for now
func preLocalChecks(app *application.Avalanche, clusterName string, anrSettings ANRSettings, avaGoVersionSettings AvalancheGoVersionSettings, avalanchegoBinaryPath string, useEtnaDevnet bool, globalNetworkFlags networkoptions.NetworkFlags) error {
	clusterExists, err := CheckClusterExists(app, clusterName)
	if err != nil {
		return fmt.Errorf("error checking cluster: %w", err)
	}
	if clusterExists {
		return fmt.Errorf("cluster %q already exists", clusterName)
	}
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
	if avalanchegoBinaryPath != "" && (avaGoVersionSettings.UseLatestAvalanchegoReleaseVersion || avaGoVersionSettings.UseLatestAvalanchegoPreReleaseVersion || avaGoVersionSettings.UseCustomAvalanchegoVersion != "") {
		return fmt.Errorf("specify either --avalanchego-path or --latest-avalanchego-version or --custom-avalanchego-version")
	}
	if useEtnaDevnet && (globalNetworkFlags.UseDevnet || globalNetworkFlags.UseFuji) {
		return fmt.Errorf("etna devnet can only be used with devnet")
	}
	if useEtnaDevnet && anrSettings.GenesisPath != "" {
		return fmt.Errorf("etna devnet uses predefined genesis file")
	}
	if useEtnaDevnet && anrSettings.UpgradePath != "" {
		return fmt.Errorf("etna devnet uses predefined upgrade file")
	}
	if useEtnaDevnet && (len(anrSettings.BootstrapIDs) != 0 || len(anrSettings.BootstrapIPs) != 0) {
		return fmt.Errorf("etna devnet uses predefined bootstrap configuration")
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
	StopLocalNode(app)

	rootDir := app.GetLocalDir(clusterName)
	if err := os.RemoveAll(rootDir); err != nil {
		return err
	}

	if ok, err := checkClusterIsLocal(app, clusterName); err != nil || !ok {
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
	bootstrapped, err := localnet.CheckNetworkIsAlreadyBootstrapped(ctx, cli)
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
