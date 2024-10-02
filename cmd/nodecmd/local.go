// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	bootstrapIDs  []string
	bootstrapIPs  []string
	genesisPath   string
	upgradePath   string
	useEtnaDevnet bool
)

func newLocalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "(ALPHA Warning) Create a new validator on local machine",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node local command sets up a validator on a local server. 
The validator will be validating the Avalanche Primary Network and Subnet 
of your choice. By default, the command runs an interactive wizard. It 
walks you through all the steps you need to set up a validator.
Once this command is completed, you will have to wait for the validator
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. You can check the bootstrapping
status by running avalanche node status local 
`,
		RunE:              localNode,
		PersistentPostRun: handlePostRun,
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, false, createSupportedNetworkOptions)
	cmd.Flags().BoolVar(&useLatestAvalanchegoReleaseVersion, "latest-avalanchego-version", false, "install latest avalanchego release version on node/s")
	cmd.Flags().BoolVar(&useLatestAvalanchegoPreReleaseVersion, "latest-avalanchego-pre-release-version", false, "install latest avalanchego pre-release version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	cmd.Flags().StringVar(&useAvalanchegoVersionFromSubnet, "avalanchego-version-from-subnet", "", "install latest avalanchego version, that is compatible with the given subnet, on node/s")
	cmd.Flags().StringArrayVar(&bootstrapIDs, "bootstrap-id", []string{}, "nodeIDs of bootstrap nodes")
	cmd.Flags().StringArrayVar(&bootstrapIPs, "bootstrap-ip", []string{}, "IP:port pairs of bootstrap nodes")
	cmd.Flags().StringVar(&genesisPath, "genesis", "", "path to genesis file")
	cmd.Flags().StringVar(&upgradePath, "upgrade", "", "path to upgrade file")
	cmd.Flags().BoolVar(&useEtnaDevnet, "etna-devnet", false, "use Etna devnet. Prepopulated with Etna DevNet bootstrap configuration along with genesis and upgrade files")
	return cmd
}

// stub for now
func preLocalChecks() error {
	// expand passed paths
	if genesisPath != "" {
		genesisPath = utils.ExpandHome(genesisPath)
	}
	if upgradePath != "" {
		upgradePath = utils.ExpandHome(upgradePath)
	}
	// checks
	if useEtnaDevnet && !globalNetworkFlags.UseDevnet || globalNetworkFlags.UseFuji {
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

func localNode(cmd *cobra.Command, args []string) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
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
	if err := preLocalChecks(); err != nil {
		return err
	}
	avalancheGoVersion, err := getAvalancheGoVersion()
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Using AvalancheGo version: %s", avalancheGoVersion)

	genesisData := []byte{}
	upgradeData := []byte{}
	if useEtnaDevnet {
		bootstrapIDs = constants.EtnaDevnetBootstrapNodeIDs
		bootstrapIPs = constants.EtnaDevnetBootstrapIPs
		genesisData = constants.EtnaDevnetGenesisData
		upgradeData = constants.EtnaDevnetUpgradeData
	} else {
		// read genesis and upgrade files if passes
		if genesisPath != "" && utils.FileExists(genesisPath) {
			genesisData, err = os.ReadFile(genesisPath)
			if err != nil {
				return fmt.Errorf("could not read genesis file %s: %w", genesisPath, err)
			}
		}
		if upgradePath != "" && utils.FileExists(upgradePath) {
			upgradeData, err = os.ReadFile(upgradePath)
			if err != nil {
				return fmt.Errorf("could not read upgrade file %s: %w", upgradePath, err)
			}
		}
	}
	bootstrapConfigNodeIDs, err := utils.StringSliceToNodeIds(bootstrapIDs)
	if err != nil {
		return fmt.Errorf("could not convert bootstrap IDs: %w", err)
	}
	bootstrapConfigNodeIPs, err := utils.StringSliceToNetipPorts(bootstrapIPs)
	if err != nil {
		return fmt.Errorf("could not convert bootstrap IP:port pairs: %w", err)
	}
	customNetwork, err := network.Customize(
		genesisData,
		upgradeData,
		bootstrapConfigNodeIDs,
		bootstrapConfigNodeIPs,
	)
	if err != nil {
		return fmt.Errorf("could not configure network: %w", err)
	}

	return nil
}
