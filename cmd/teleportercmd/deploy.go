// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/spf13/cobra"
)

type DeployFlags struct {
	Network      networkoptions.NetworkFlags
	BlockchainID string
	PrivateKey   string
}

var (
	deploySupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
		networkoptions.Mainnet,
	}
	deployFlags DeployFlags
)

// avalanche teleporter deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [subnetName]",
		Short: "Deploys Teleporter into a given Network and Blockchain",
		Long:  `Deploys Teleporter into a given Network and Blockchain.`,
		RunE:  deploy,
		Args:  cobrautils.MaximumNArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, deploySupportedNetworkOptions)
	cmd.Flags().StringVar(&deployFlags.BlockchainID, "blockchain-id", "", "blockchain ID to deploy teleporter into (if not providing subnetName)")
	cmd.Flags().StringVar(&deployFlags.PrivateKey, "private-key", "", "private key to use to fund teleporter deploy)")
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args, deployFlags)
}

func CallDeploy(args []string, flags DeployFlags) error {
	var subnetName string
	if len(args) == 1 {
		subnetName = args[0]
	}
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"On what Network do you want to deploy the Teleporter Messenger?",
		flags.Network,
		true,
		false,
		deploySupportedNetworkOptions,
		subnetName,
	)
	if err != nil {
		return err
	}
	if subnetName != "" && flags.BlockchainID != "" {
		return fmt.Errorf("subnetName and blockchainID are mutually exclusive cmdline options")
	}
	var (
		blockchainID ids.ID
		privateKey   = flags.PrivateKey
	)
	if flags.BlockchainID != "" {
		blockchainID, err = ids.FromString(flags.BlockchainID)
		if err != nil {
			return fmt.Errorf("invalid blockchain id %s: %w", flags.BlockchainID, err)
		}
	}
	if subnetName == "" && flags.BlockchainID == "" {
		blockchainIDOptions := []string{
			"Grab it from a CLI Subnet",
			"I will provide a Custom one",
		}
		if blockchainIDOption, err := app.Prompt.CaptureList("Which is the Blockchain ID?", blockchainIDOptions); err != nil {
			return err
		} else if blockchainIDOption == blockchainIDOptions[0] {
			subnetNames, err := app.GetSubnetNames()
			if err != nil {
				return err
			}
			subnetName, err = app.Prompt.CaptureList(
				"Choose a Subnet",
				subnetNames,
			)
			if err != nil {
				return err
			}
		} else {
			blockchainID, err = app.Prompt.CaptureID("Blockchain ID")
			if err != nil {
				return err
			}
		}
	}
	var teleporterVersion string
	if subnetName != "" {
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
		blockchainID = sc.Networks[network.Name()].BlockchainID
		if sc.TeleporterVersion != "" {
			teleporterVersion = sc.TeleporterVersion
		}
		if sc.TeleporterKey != "" {
			k, err := app.GetKey(sc.TeleporterKey, network, true)
			if err != nil {
				return nil
			}
			privateKey = k.Hex()
		}
	} else {
		createChainTx, err := utils.GetBlockchainTx(network.Endpoint, blockchainID)
		if err != nil {
			return err
		}
		if !utils.ByteSliceIsSubnetEvmGenesis(createChainTx.GenesisData) {
			return fmt.Errorf("only Subnet-EVM based vms can be used for teleporter, blockchain genesis is not")
		}
	}
	if privateKey == "" {
		keyOptions := []string{
			"Grab it from a CLI Stored Key",
			"I will provide a Custom one",
		}
		if keyOption, err := app.Prompt.CaptureList("Which Private Key to use to pay fees?", keyOptions); err != nil {
			return err
		} else if keyOption == keyOptions[0] {
			keyName, err := prompts.CaptureKeyName(app.Prompt, "pay fees", app.GetKeyDir(), true)
			if err != nil {
				return err
			}
			k, err := app.GetKey(keyName, network, false)
			if err != nil {
				return nil
			}
			privateKey = k.Hex()
		} else {
			privateKey, err = app.Prompt.CaptureString("Private Key")
			if err != nil {
				return err
			}
		}
	}
	fmt.Println(blockchainID)
	fmt.Println(privateKey)
	return nil
	// deploy to subnet
	alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := teleporter.DeployAndFundRelayer(
		app,
		teleporterVersion,
		network,
		subnetName,
		blockchainID.String(),
		privateKey,
	)
	if err != nil {
		return err
	}
	if subnetName != "" && !alreadyDeployed {
		// update sidecar
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return fmt.Errorf("failed to load sidecar: %w", err)
		}
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
			teleporterVersion,
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
