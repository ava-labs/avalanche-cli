// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/spf13/cobra"
)

var deploySupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

// avalanche teleporter deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [subnetName]",
		Short: "Deploys Teleporter into a given Network and Blockchain",
		Long:  `Deploys Teleporter into a given Network and Blockchain.`,
		RunE:  deploy,
		Args:  cobrautils.MaximumNArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, deploySupportedNetworkOptions)
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args, globalNetworkFlags)
}

func CallDeploy(args []string, flags networkoptions.NetworkFlags) error {
	var subnetName string
	if len(args) == 1 {
		subnetName = args[0]
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"On what Network do you want to deploy the Teleporter Messenger?",
		flags,
		true,
		false,
		deploySupportedNetworkOptions,
		subnetName,
	)
	if err != nil {
		return err
	}
	if subnetName == "" {
		if yes, err := app.Prompt.CaptureYesNo("Do you have a CLI Subnet associated with the Blockchain?"); err != nil {
			return err
		} else if yes {
			fmt.Println(app.GetSubnetNames())
		}
		// ask for either subnet name or subnet chain id
		// if subnet name, everything ok, except key
		// if not:
		// get genesis from chain id and validate
		// ask for a key to do operations if not already given
	}
	return nil
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}
	// checks
	if b, _, err := subnetcmd.HasSubnetEVMGenesis(subnetName); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("only Subnet-EVM based vms can be used for teleporter")
	}
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("subnet has not been deployed to %s", network.Name())
	}
	if !sc.TeleporterReady {
		return fmt.Errorf("subnet is not configured for teleporter")
	}
	// deploy to subnet
	blockchainID := sc.Networks[network.Name()].BlockchainID.String()
	alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := teleporter.DeployAndFundRelayer(
		app,
		sc.TeleporterVersion,
		network,
		subnetName,
		blockchainID,
		sc.TeleporterKey,
	)
	if err != nil {
		return err
	}
	if !alreadyDeployed {
		// update sidecar
		networkInfo := sc.Networks[network.Name()]
		networkInfo.TeleporterMessengerAddress = teleporterMessengerAddress
		networkInfo.TeleporterRegistryAddress = teleporterRegistryAddress
		sc.Networks[network.Name()] = networkInfo
		if err := app.UpdateSidecar(&sc); err != nil {
			return err
		}
	}
	// deploy to cchain for local
	if network.Kind == models.Local || network.Kind == models.Devnet {
		blockchainID := "C"
		alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err = teleporter.DeployAndFundRelayer(
			app,
			sc.TeleporterVersion,
			network,
			"c-chain",
			blockchainID,
			"",
		)
		if err != nil {
			return err
		}
		if !alreadyDeployed {
			if network.Kind == models.Local {
				if err := subnet.WriteExtraLocalNetworkData(app, teleporterMessengerAddress, teleporterRegistryAddress); err != nil {
					return err
				}
			}
			if network.ClusterName != "" {
				clusterConfig, err := app.GetClusterConfig(network.ClusterName)
				if err != nil {
					return err
				}
				clusterConfig.ExtraNetworkData = models.ExtraNetworkData{
					CChainTeleporterMessengerAddress: teleporterMessengerAddress,
					CChainTeleporterRegistryAddress:  teleporterRegistryAddress,
				}
				if err := app.SetClusterConfig(network.ClusterName, clusterConfig); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
