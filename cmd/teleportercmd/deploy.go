// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
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
	SubnetName   string
	BlockchainID string
	CChain       bool
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
		Use:   "deploy",
		Short: "Deploys Teleporter into a given Network and Blockchain",
		Long:  `Deploys Teleporter into a given Network and Blockchain.`,
		RunE:  deploy,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, deploySupportedNetworkOptions)
	cmd.Flags().StringVar(&deployFlags.SubnetName, "subnet", "", "deploy teleporter into the given CLI subnet")
	cmd.Flags().StringVar(&deployFlags.BlockchainID, "blockchain-id", "", "deploy teleporter into the given blockchain ID")
	cmd.Flags().BoolVar(&deployFlags.CChain, "c-chain", false, "deploy teleporter into C-Chain")
	cmd.Flags().StringVar(&deployFlags.PrivateKey, "private-key", "", "private key to use to fund teleporter deploy)")
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args, deployFlags)
}

func CallDeploy(_ []string, flags DeployFlags) error {
	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"On what Network do you want to deploy the Teleporter Messenger?",
		flags.Network,
		true,
		false,
		deploySupportedNetworkOptions,
		flags.SubnetName,
	)
	if err != nil {
		return err
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.SubnetName != "", flags.BlockchainID != "", flags.CChain}) {
		return fmt.Errorf("--subnet, --blockchain-id and --cchain are mutually exclusive flags")
	}
	var (
		blockchainAlias string
		blockchainID    ids.ID
		subnetName      = flags.SubnetName
		privateKey      = flags.PrivateKey
	)
	if flags.CChain {
		blockchainAlias = "C"
	}
	if flags.BlockchainID != "" {
		blockchainID, err = ids.FromString(flags.BlockchainID)
		if err != nil {
			return fmt.Errorf("invalid blockchain id %s: %w", flags.BlockchainID, err)
		}
	}
	if subnetName == "" && flags.BlockchainID == "" && !flags.CChain {
		blockchainIDOptions := []string{
			"Get it from a CLI Subnet",
			"Use C-Chain",
			"Will provide a Custom one",
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
		} else if blockchainIDOption == blockchainIDOptions[1] {
			blockchainAlias = "C"
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
	} else if flags.BlockchainID != "" {
		createChainTx, err := utils.GetBlockchainTx(network.Endpoint, blockchainID)
		if err != nil {
			return err
		}
		if !utils.ByteSliceIsSubnetEvmGenesis(createChainTx.GenesisData) {
			return fmt.Errorf("only Subnet-EVM based vms can be used for teleporter, blockchain genesis is not")
		}
	}
	if blockchainAlias == "" {
		blockchainAlias = blockchainID.String()
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
	fmt.Println(blockchainAlias)
	fmt.Println(privateKey)
	return nil
	// deploy to subnet
	alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := teleporter.DeployAndFundRelayer(
		app,
		teleporterVersion,
		network,
		subnetName,
		blockchainAlias,
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
