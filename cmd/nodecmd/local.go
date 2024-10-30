// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
)

var (
	avalanchegoBinaryPath string

	bootstrapIDs         []string
	bootstrapIPs         []string
	genesisPath          string
	upgradePath          string
	stakingTLSKeyPath    string
	stakingCertKeyPath   string
	stakingSignerKeyPath string
	numNodes             uint32
	nodeConfigPath       string
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
	// node local status
	cmd.AddCommand(newLocalStatusCmd())
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
	cmd.Flags().StringVar(&stakingTLSKeyPath, "staking-tls-key-path", "", "path to provided staking tls key for node")
	cmd.Flags().StringVar(&stakingCertKeyPath, "staking-cert-key-path", "", "path to provided staking cert key for node")
	cmd.Flags().StringVar(&stakingSignerKeyPath, "staking-signer-key-path", "", "path to provided staking signer key for node")
	cmd.Flags().Uint32Var(&numNodes, "num-nodes", 1, "number of nodes to start")
	cmd.Flags().StringVar(&nodeConfigPath, "node-config", "", "path to common avalanchego config settings for all nodes")
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
	cmd := &cobra.Command{
		Use:   "track [clusterName] [blockchainName]",
		Short: "(ALPHA Warning) make the local node at the cluster to track given blockchain",
		Long:  "(ALPHA Warning) make the local node at the cluster to track given blockchain",
		Args:  cobra.ExactArgs(2),
		RunE:  localTrack,
	}
	cmd.Flags().StringVar(&avalanchegoBinaryPath, "avalanchego-path", "", "use this avalanchego binary path")
	return cmd
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

func newLocalStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "(ALPHA Warning) Get status of local node",
		Long:  `Get status of local node.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  localStatus,
	}

	cmd.Flags().StringVar(&blockchainName, "subnet", "", "specify the blockchain the node is syncing with")
	cmd.Flags().StringVar(&blockchainName, "blockchain", "", "specify the blockchain the node is syncing with")

	return cmd
}

func localStartNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	anrSettings := node.ANRSettings{
		GenesisPath:          genesisPath,
		UpgradePath:          upgradePath,
		BootstrapIDs:         bootstrapIDs,
		BootstrapIPs:         bootstrapIPs,
		StakingSignerKeyPath: stakingTLSKeyPath,
		StakingCertKeyPath:   stakingCertKeyPath,
		StakingTLSKeyPath:    stakingTLSKeyPath,
	}
	avaGoVersionSetting := node.AvalancheGoVersionSettings{
		UseCustomAvalanchegoVersion:           useCustomAvalanchegoVersion,
		UseLatestAvalanchegoPreReleaseVersion: useLatestAvalanchegoPreReleaseVersion,
		UseLatestAvalanchegoReleaseVersion:    useLatestAvalanchegoReleaseVersion,
		UseAvalanchegoVersionFromSubnet:       useAvalanchegoVersionFromSubnet,
	}
	nodeConfig := ""
	if nodeConfigPath != "" {
		nodeConfigBytes, err := os.ReadFile(nodeConfigPath)
		if err != nil {
			return err
		}
		nodeConfig = string(nodeConfigBytes)
	}
	return node.StartLocalNode(
		app,
		clusterName,
		globalNetworkFlags.UseEtnaDevnet,
		avalanchegoBinaryPath,
		numNodes,
		nodeConfig,
		anrSettings,
		avaGoVersionSetting,
		globalNetworkFlags,
		createSupportedNetworkOptions,
	)
}

func localStopNode(_ *cobra.Command, _ []string) error {
	return node.StopLocalNode(app)
}

func localDestroyNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	return node.DestroyLocalNode(app, clusterName)
}

func localTrack(_ *cobra.Command, args []string) error {
	return node.TrackSubnetWithLocalMachine(app, args[0], args[1], avalanchegoBinaryPath)
}

func localStatus(_ *cobra.Command, args []string) error {
	clusterName := ""
	if len(args) > 0 {
		clusterName = args[0]
	}
	if blockchainName != "" && clusterName == "" {
		return fmt.Errorf("--blockchain flag is only supported if clusterName is specified")
	}
	return node.LocalStatus(app, clusterName, blockchainName)
}

func notImplementedForLocal(what string) error {
	ux.Logger.PrintToUser("Unsupported cmd: %s is not supported by local clusters", logging.LightBlue.Wrap(what))
	return nil
}
