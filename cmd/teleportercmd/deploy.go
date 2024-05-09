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
	KeyName      string
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
	cmd.Flags().StringVar(&deployFlags.BlockchainID, "blockchain-id", "", "deploy teleporter into the given blockchain ID/Alias")
	cmd.Flags().BoolVar(&deployFlags.CChain, "c-chain", false, "deploy teleporter into C-Chain")
	cmd.Flags().StringVar(&deployFlags.PrivateKey, "private-key", "", "private key to use to fund teleporter deploy)")
	cmd.Flags().StringVar(&deployFlags.KeyName, "key", "", "CLI stored key to use to fund teleporter deploy)")
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
	if flags.SubnetName == "" && flags.BlockchainID == "" && !flags.CChain {
		// fill flags based on user prompts
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
			flags.SubnetName, err = app.Prompt.CaptureList(
				"Choose a Subnet",
				subnetNames,
			)
			if err != nil {
				return err
			}
		} else if blockchainIDOption == blockchainIDOptions[1] {
			flags.CChain = true
		} else {
			flags.BlockchainID, err = app.Prompt.CaptureString("Blockchain ID/Alias")
			if err != nil {
				return err
			}
		}
	}

	var (
		teleporterVersion string
		blockchainID      string
		privateKey        = flags.PrivateKey
	)
	switch {
	case flags.SubnetName != "":
		sc, err := app.LoadSidecar(flags.SubnetName)
		if err != nil {
			return fmt.Errorf("failed to load sidecar: %w", err)
		}
		if b, _, err := subnetcmd.HasSubnetEVMGenesis(flags.SubnetName); err != nil {
			return err
		} else if !b {
			return fmt.Errorf("only Subnet-EVM based vms can be used for teleporter")
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return fmt.Errorf("subnet has not been deployed to %s", network.Name())
		}
		blockchainID = sc.Networks[network.Name()].BlockchainID.String()
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
	case flags.BlockchainID != "":
		chainID, err := subnet.GetChainID(network, flags.BlockchainID)
		if err != nil {
			return err
		}
		createChainTx, err := utils.GetBlockchainTx(network.Endpoint, chainID)
		if err != nil {
			return err
		}
		if !utils.ByteSliceIsSubnetEvmGenesis(createChainTx.GenesisData) {
			return fmt.Errorf("only Subnet-EVM based vms can be used for teleporter, blockchain genesis is not")
		}
		blockchainID = flags.BlockchainID
	case flags.CChain:
		blockchainID = "C"
	}
	if flags.KeyName != "" {
		k, err := app.GetKey(flags.KeyName, network, false)
		if err != nil {
			return nil
		}
		privateKey = k.Hex()
	}
	if privateKey == "" {
		keyOptions := []string{
			"Get it from a CLI Key",
			"Will provide a Custom one",
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
		flags.SubnetName,
		blockchainID,
		privateKey,
	)
	if err != nil {
		return err
	}
	if flags.SubnetName != "" && !alreadyDeployed {
		// update sidecar
		sc, err := app.LoadSidecar(flags.SubnetName)
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
