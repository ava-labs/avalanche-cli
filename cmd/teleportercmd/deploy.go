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

var deploySupportedNetworkOptions = []networkoptions.NetworkOption{networkoptions.Local, networkoptions.Cluster, networkoptions.Fuji, networkoptions.Mainnet, networkoptions.Devnet}

// avalanche teleporter deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [subnetName]",
		Short: "Deploys Teleporter into the given Subnet",
		Long:  `Deploys Teleporter into the given Subnet.`,
		RunE:  deploy,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, deploySupportedNetworkOptions)
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args[0], globalNetworkFlags)
}

func CallDeploy(subnetName string, flags networkoptions.NetworkFlags) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		flags,
		true,
		deploySupportedNetworkOptions,
		subnetName,
	)
	if err != nil {
		return err
	}
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}
	// checks
	if !sc.TeleporterReady {
		return fmt.Errorf("subnet is not configured for teleporter")
	}
	if b, err := subnetcmd.HasSubnetEVMGenesis(subnetName); err != nil {
		return err
	} else if !b {
		return fmt.Errorf("only Subnet-EVM based vms can be used for teleporter")
	}
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("subnet has not been deployed to %s", network.Name())
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
