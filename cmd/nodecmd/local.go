// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
)

var (
	avalanchegoBinaryPath string

	bootstrapIDs         []string
	bootstrapIPs         []string
	genesisPath          string
	upgradePath          string
	useEtnaDevnet        bool
	stakingTLSKeyPath    string
	stakingCertKeyPath   string
	stakingSignerKeyPath string
)

// const snapshotName = "local_snapshot"
func newLocalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "(ALPHA Warning) Suite of commands for a local avalanche node",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node local command suite provides a collection of commands related to local nodes`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	// node local start
	cmd.AddCommand(newLocalStartCmd())
	// node local stop
	cmd.AddCommand(newLocalStopCmd())
	// node local destroy
	cmd.AddCommand(newLocalDestroyCmd())
	// node local track
	cmd.AddCommand(newLocalTrackCmd())
	return cmd
}

func newLocalStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [clusterName]",
		Short: "(ALPHA Warning) Create a new validator on local machine",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node local start command sets up a validator on a local server. 
The validator will be validating the Avalanche Primary Network and Subnet 
of your choice. By default, the command runs an interactive wizard. It 
walks you through all the steps you need to set up a validator.
Once this command is completed, you will have to wait for the validator
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. You can check the bootstrapping
status by running avalanche node status local 
`,
		Args:              cobra.ExactArgs(1),
		RunE:              localStartNode,
		PersistentPostRun: handlePostRun,
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, createSupportedNetworkOptions)
	cmd.Flags().BoolVar(&useLatestAvalanchegoReleaseVersion, "latest-avalanchego-version", false, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&useLatestAvalanchegoPreReleaseVersion, "latest-avalanchego-pre-release-version", false, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	cmd.Flags().StringVar(&avalanchegoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	cmd.Flags().StringArrayVar(&bootstrapIDs, "bootstrap-id", []string{}, "nodeIDs of bootstrap nodes")
	cmd.Flags().StringArrayVar(&bootstrapIPs, "bootstrap-ip", []string{}, "IP:port pairs of bootstrap nodes")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "path to genesis file")
	cmd.Flags().StringVar(&upgradePath, "upgrade", "", "path to upgrade file")
	cmd.Flags().BoolVar(&useEtnaDevnet, "etna-devnet", false, "use Etna devnet. Prepopulated with Etna DevNet bootstrap configuration along with genesis and upgrade files")
	cmd.Flags().StringVar(&stakingTLSKeyPath, "staking-tls-key-path", "", "path to provided staking tls key for node")
	cmd.Flags().StringVar(&stakingCertKeyPath, "staking-cert-key-path", "", "path to provided staking cert key for node")
	cmd.Flags().StringVar(&stakingSignerKeyPath, "staking-signer-key-path", "", "path to provided staking signer key for node")
	return cmd
}

func newLocalStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "(ALPHA Warning) Stop local node",
		Long:  `Stop local node.`,
		Args:  cobra.ExactArgs(0),
		RunE:  localStopNode,
	}
}

func newLocalTrackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "track [clusterName] [blockchainName]",
		Short: "(ALPHA Warning) make the local node at the cluster to track given blockchain",
		Long:  "(ALPHA Warning) make the local node at the cluster to track given blockchain",
		Args:  cobra.ExactArgs(2),
		RunE:  localTrack,
	}
}

func newLocalDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy [clusterName]",
		Short: "(ALPHA Warning) Cleanup local node",
		Long:  `Cleanup local node.`,
		Args:  cobra.ExactArgs(1),
		RunE:  localDestroyNode,
	}
}

// stub for now
func preLocalChecks(clusterName string) error {
	clusterExists, err := checkClusterExists(clusterName)
	if err != nil {
		return fmt.Errorf("error checking cluster: %w", err)
	}
	if clusterExists {
		return fmt.Errorf("cluster %q already exists", clusterName)
	}
	// expand passed paths
	if genesisPath != "" {
		genesisPath = utils.ExpandHome(genesisPath)
	}
	if upgradePath != "" {
		upgradePath = utils.ExpandHome(upgradePath)
	}
	// checks
	if useCustomAvalanchegoVersion != "" && (useLatestAvalanchegoReleaseVersion || useLatestAvalanchegoPreReleaseVersion) {
		return fmt.Errorf("specify either --custom-avalanchego-version or --latest-avalanchego-version")
	}
	if avalanchegoBinaryPath != "" && (useLatestAvalanchegoReleaseVersion || useLatestAvalanchegoPreReleaseVersion || useCustomAvalanchegoVersion != "") {
		return fmt.Errorf("specify either --avalanchego-path or --latest-avalanchego-version or --custom-avalanchego-version")
	}
	if useEtnaDevnet && (globalNetworkFlags.UseDevnet || globalNetworkFlags.UseFuji) {
		return fmt.Errorf("etna devnet can only be used with devnet")
	}
	if useEtnaDevnet && genesisPath != "" {
		return fmt.Errorf("etna devnet uses predefined genesis file")
	}
	if useEtnaDevnet && upgradePath != "" {
		return fmt.Errorf("etna devnet uses predefined upgrade file")
	}
	if useEtnaDevnet && (len(bootstrapIDs) != 0 || len(bootstrapIPs) != 0) {
		return fmt.Errorf("etna devnet uses predefined bootstrap configuration")
	}
	if len(bootstrapIDs) != len(bootstrapIPs) {
		return fmt.Errorf("number of bootstrap IDs and bootstrap IP:port pairs must be equal")
	}
	if genesisPath != "" && !utils.FileExists(genesisPath) {
		return fmt.Errorf("genesis file %s does not exist", genesisPath)
	}
	if upgradePath != "" && !utils.FileExists(upgradePath) {
		return fmt.Errorf("upgrade file %s does not exist", upgradePath)
	}
	return nil
}

func localClusterDataExists(clusterName string) bool {
	rootDir := app.GetLocalDir(clusterName)
	return utils.FileExists(filepath.Join(rootDir, "state.json"))
}

func localStartNode(_ *cobra.Command, args []string) error {
	var err error
	clusterName := args[0]
	network := models.UndefinedNetwork

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
		avalancheGoVersion, err = getAvalancheGoVersion()
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

	if localClusterDataExists(clusterName) {
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
		if err := preLocalChecks(clusterName); err != nil {
			return err
		}
		if useEtnaDevnet {
			bootstrapIDs = constants.EtnaDevnetBootstrapNodeIDs
			bootstrapIPs = constants.EtnaDevnetBootstrapIPs
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
			genesisPath = genesisFile.Name()
			defer os.Remove(genesisPath)

			upgradeFile, err := os.CreateTemp("", "etna_devnet_upgrade")
			if err != nil {
				return fmt.Errorf("could not create save Etna Devnet upgrade file: %w", err)
			}
			if _, err := upgradeFile.Write(constants.EtnaDevnetUpgradeData); err != nil {
				return fmt.Errorf("could not write Etna Devnet upgrade data: %w", err)
			}
			upgradePath = upgradeFile.Name()
			if err := upgradeFile.Close(); err != nil {
				return fmt.Errorf("could not close Etna Devnet upgrade file: %w", err)
			}
			defer os.Remove(upgradePath)
		}

		if stakingTLSKeyPath != "" && stakingCertKeyPath != "" && stakingSignerKeyPath != "" {
			if err := os.MkdirAll(filepath.Join(rootDir, "node1", "staking"), 0o700); err != nil {
				return fmt.Errorf("could not create root directory %s: %w", rootDir, err)
			}
			if err := utils.FileCopy(stakingTLSKeyPath, filepath.Join(rootDir, "node1", "staking", "staker.key")); err != nil {
				return err
			}
			if err := utils.FileCopy(stakingCertKeyPath, filepath.Join(rootDir, "node1", "staking", "staker.crt")); err != nil {
				return err
			}
			if err := utils.FileCopy(stakingSignerKeyPath, filepath.Join(rootDir, "node1", "staking", "signer.key")); err != nil {
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
		if genesisPath != "" && utils.FileExists(genesisPath) {
			anrOpts = append(anrOpts, client.WithGenesisPath(genesisPath))
		}
		if upgradePath != "" && utils.FileExists(upgradePath) {
			anrOpts = append(anrOpts, client.WithUpgradePath(upgradePath))
		}
		if bootstrapIDs != nil {
			anrOpts = append(anrOpts, client.WithBootstrapNodeIDs(bootstrapIDs))
		}
		if bootstrapIPs != nil {
			anrOpts = append(anrOpts, client.WithBootstrapNodeIPPortPairs(bootstrapIPs))
		}

		ux.Logger.PrintToUser("Starting local avalanchego node using root: %s ...", rootDir)
		spinSession := ux.NewUserSpinner()
		spinner := spinSession.SpinToUser("Booting Network. Wait until healthy...")
		if _, err := cli.Start(ctx, avalancheGoBinPath, anrOpts...); err != nil {
			ux.SpinFailWithError(spinner, "", err)
			localDestroyNode(nil, []string{clusterName})
			return fmt.Errorf("failed to start local avalanchego: %w", err)
		}
		ux.SpinComplete(spinner)
		// save cluster config for the new local cluster
		if err := addLocalClusterConfig(network); err != nil {
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

func localStopNode(_ *cobra.Command, _ []string) error {
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

func localDestroyNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]

	localStopNode(nil, nil)

	rootDir := app.GetLocalDir(clusterName)
	if err := os.RemoveAll(rootDir); err != nil {
		return err
	}

	if ok, err := checkClusterIsLocal(clusterName); err != nil || !ok {
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

func addLocalClusterConfig(network models.Network) error {
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

func checkClusterIsLocal(clusterName string) (bool, error) {
	clustersConfig, err := app.GetClustersConfig()
	if err != nil {
		return false, err
	}
	clusterConf, ok := clustersConfig.Clusters[clusterName]
	return ok && clusterConf.Local, nil
}

func localTrack(_ *cobra.Command, args []string) error {
	return node.TrackSubnetWithLocalMachine(app, args[0], args[1])
}

func notImplementedForLocal(what string) error {
	ux.Logger.PrintToUser("Unsupported cmd: %s is not supported by local clusters", logging.LightBlue.Wrap(what))
	return nil
}
