// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

func TrackSubnetWithLocalMachine(
	app *application.Avalanche,
	clusterName,
	blockchainName string,
	avalancheGoBinPath string,
) error {
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
	publicEndpoints := []string{}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		if app.ChainConfigExists(blockchainName) {
			inputChainConfigPath := app.GetChainConfigPath(blockchainName)
			outputChainConfigPath := filepath.Join(rootDir, nodeInfo.Name, "configs", "chains", blockchainID.String(), "config.json")
			if err := os.MkdirAll(filepath.Dir(outputChainConfigPath), 0o700); err != nil {
				return fmt.Errorf("could not create chain conf directory %s: %w", filepath.Dir(outputChainConfigPath), err)
			}
			if err := utils.FileCopy(inputChainConfigPath, outputChainConfigPath); err != nil {
				return err
			}
		}
		ux.Logger.PrintToUser("Restarting node %s to track subnet", nodeInfo.Name)
		opts := []client.OpOption{
			client.WithWhitelistedSubnets(subnetID.String()),
			client.WithExecPath(avalancheGoBinPath),
		}
		if _, err := cli.RestartNode(ctx, nodeInfo.Name, opts...); err != nil {
			return err
		}
		publicEndpoints = append(publicEndpoints, nodeInfo.Uri)
	}
	networkInfo := sc.Networks[network.Name()]
	rpcEndpoints := set.Of(networkInfo.RPCEndpoints...)
	wsEndpoints := set.Of(networkInfo.WSEndpoints...)
	for _, publicEndpoint := range publicEndpoints {
		rpcEndpoints.Add(models.GetRPCEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
		wsEndpoints.Add(models.GetWSEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
	}
	networkInfo.RPCEndpoints = rpcEndpoints.List()
	networkInfo.WSEndpoints = wsEndpoints.List()
	for _, rpcURL := range networkInfo.RPCEndpoints {
		ux.Logger.PrintToUser("Waiting for rpc %s to be available", rpcURL)
		if err := evm.WaitForRPC(ctx, rpcURL); err != nil {
			return err
		}
	}
	sc.Networks[network.Name()] = networkInfo
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

func StartLocalNode(
	app *application.Avalanche,
	clusterName string,
	useEtnaDevnet bool,
	avalanchegoBinaryPath string,
	numNodes uint32,
	partialSync bool,
	nodeConfig map[string]interface{},
	anrSettings ANRSettings,
	avaGoVersionSetting AvalancheGoVersionSettings,
	globalNetworkFlags networkoptions.NetworkFlags,
	createSupportedNetworkOptions []networkoptions.NetworkOption,
) error {
	var err error

	// ensure data consistency
	localClusterExists := false
	if clusterExists, err := CheckClusterExists(app, clusterName); err != nil {
		return fmt.Errorf("error checking clusters info: %w", err)
	} else if clusterExists {
		if localClusterExists, err = checkClusterIsLocal(app, clusterName); err != nil {
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
	if partialSync {
		nodeConfig[config.PartialSyncPrimaryNetworkKey] = true
	}
	nodeConfig[config.NetworkAllowPrivateIPsKey] = true

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
		network := models.UndefinedNetwork
		switch {
		case useEtnaDevnet:
			network = models.NewNetwork(
				models.Devnet,
				constants.EtnaDevnetNetworkID,
				constants.EtnaDevnetEndpoint,
				clusterName,
			)
		case globalNetworkFlags.UseFuji:
			network = models.NewNetwork(
				models.Fuji,
				avagoconstants.FujiID,
				constants.FujiAPIEndpoint,
				clusterName,
			)
		default:
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
		if network.Kind == models.Fuji {
			ux.Logger.PrintToUser(logging.Yellow.Wrap("Warning: Fuji Bootstrapping can take several minutes"))
		}
		if err := preLocalChecks(anrSettings, avaGoVersionSetting, useEtnaDevnet, globalNetworkFlags); err != nil {
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
func preLocalChecks(anrSettings ANRSettings, avaGoVersionSettings AvalancheGoVersionSettings, useEtnaDevnet bool, globalNetworkFlags networkoptions.NetworkFlags) error {
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
	_ = StopLocalNode(app)

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
			if ok, err := checkClusterIsLocal(app, clusterName); err == nil && ok {
				localClusters[clusterName] = app.GetLocalDir(clusterName)
			}
		}
	}
	return localClusters, nil
}

func LocalStatus(app *application.Avalanche, clusterName string, blockchainName string) error {
	clustersToList := make([]string, 0)
	if clusterName != "" {
		if ok, err := checkClusterIsLocal(app, clusterName); err != nil || !ok {
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

		// load sidecar and cluster config for the cluster  if blockchainName is not empty
		blockchainID := ids.Empty
		if blockchainName != "" {
			clusterConf, err := app.GetClusterConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to get cluster config: %w", err)
			}
			sc, err := app.LoadSidecar(blockchainName)
			if err != nil {
				return err
			}
			network := models.ConvertClusterToNetwork(clusterConf.Network)
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
		ux.Logger.PrintToUser("- %s: %s %s %s", clusterName, rootDir, currenlyRunning, healthStatus)
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
