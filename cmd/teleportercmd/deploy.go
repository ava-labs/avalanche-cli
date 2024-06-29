// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/spf13/cobra"
)

type DeployFlags struct {
	Network                      networkoptions.NetworkFlags
	SubnetName                   string
	BlockchainID                 string
	CChain                       bool
	PrivateKey                   string
	KeyName                      string
	GenesisKey                   bool
	DeployMessenger              bool
	DeployRegistry               bool
	RPCURL                       string
	Version                      string
	MessengerContractAddressPath string
	MessengerDeployerAddressPath string
	MessengerDeployerTxPath      string
	RegistryBydecodePath         string
	PrivateKeyFlags              PrivateKeyFlags
}

const (
	cChainAlias = "C"
	cChainName  = "c-chain"
)

var (
	deploySupportedNetworkOptions = []networkoptions.NetworkOption{
		networkoptions.Local,
		networkoptions.Devnet,
		networkoptions.Fuji,
	}
	deployFlags DeployFlags
)

// avalanche teleporter deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys Teleporter into a given Network and Subnet",
		Long:  `Deploys Teleporter into a given Network and Subnet.`,
		RunE:  deploy,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, deploySupportedNetworkOptions)
	cmd.Flags().StringVar(&deployFlags.SubnetName, "subnet", "", "deploy teleporter into the given CLI subnet")
	cmd.Flags().StringVar(&deployFlags.BlockchainID, "blockchain-id", "", "deploy teleporter into the given blockchain ID/Alias")
	cmd.Flags().BoolVar(&deployFlags.CChain, "c-chain", false, "deploy teleporter into C-Chain")
	cmd.Flags().StringVar(&deployFlags.PrivateKeyFlags.PrivateKey, "private-key", "", "private key to use to fund teleporter deploy)")
	cmd.Flags().StringVar(&deployFlags.PrivateKeyFlags.KeyName, "key", "", "CLI stored key to use to fund teleporter deploy)")
	cmd.Flags().BoolVar(&deployFlags.PrivateKeyFlags.GenesisKey, "genesis-key", false, "use genesis aidrop key to fund teleporter deploy")
	cmd.Flags().BoolVar(&deployFlags.DeployMessenger, "deploy-messenger", true, "deploy Teleporter Messenger")
	cmd.Flags().BoolVar(&deployFlags.DeployRegistry, "deploy-registry", true, "deploy Teleporter Registry")
	cmd.Flags().StringVar(&deployFlags.RPCURL, "rpc-url", "", "use the given RPC URL to connect to the subnet")
	cmd.Flags().StringVar(&deployFlags.Version, "version", "latest", "version to deploy")
	cmd.Flags().StringVar(&deployFlags.MessengerContractAddressPath, "messenger-contract-address-path", "", "path to a messenger contract address file")
	cmd.Flags().StringVar(&deployFlags.MessengerDeployerAddressPath, "messenger-deployer-address-path", "", "path to a messenger deployer address file")
	cmd.Flags().StringVar(&deployFlags.MessengerDeployerTxPath, "messenger-deployer-tx-path", "", "path to a messenger deployer tx file")
	cmd.Flags().StringVar(&deployFlags.RegistryBydecodePath, "registry-bytecode-path", "", "path to a registry bytecode file")
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
		"",
	)
	if err != nil {
		return err
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.SubnetName != "", flags.BlockchainID != "", flags.CChain}) {
		return fmt.Errorf("--subnet, --blockchain-id and --cchain are mutually exclusive flags")
	}
	if !cmdflags.EnsureMutuallyExclusive([]bool{flags.PrivateKey != "", flags.KeyName != "", flags.GenesisKey}) {
		return fmt.Errorf("--private-key, --key and --genesis-key are mutually exclusive flags")
	}
	if !flags.DeployMessenger && !flags.DeployRegistry {
		return fmt.Errorf("you should set at least one of --deploy-messenger/--deploy-registry to true")
	}
	if flags.SubnetName == "" && flags.BlockchainID == "" && !flags.CChain {
		// fill flags based on user prompts
		blockchainIDOptions := []string{
			"Get Blockchain ID from an existing subnet (deployed with avalanche subnet deploy)",
			"Use C-Chain Blockchain ID",
			"Custom",
		}
		blockchainIDOption, err := app.Prompt.CaptureList("Which Blockchain ID would you like to deploy Teleporter to?", blockchainIDOptions)
		if err != nil {
			return err
		}
		switch blockchainIDOption {
		case blockchainIDOptions[0]:
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
		case blockchainIDOptions[1]:
			flags.CChain = true
		default:
			flags.BlockchainID, err = app.Prompt.CaptureString("Blockchain ID/Alias")
			if err != nil {
				return err
			}
		}
	}

	var (
		blockchainID         string
		teleporterSubnetDesc string
		privateKey           string
		teleporterVersion    string
	)
	switch {
	case flags.SubnetName != "":
		teleporterSubnetDesc = flags.SubnetName
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
				return err
			}
			privateKey = k.PrivKeyHex()
		}
	case flags.BlockchainID != "":
		teleporterSubnetDesc = flags.BlockchainID
		blockchainID = flags.BlockchainID
	case flags.CChain:
		teleporterSubnetDesc = cChainName
		blockchainID = cChainAlias
	}
	genesisAddress, genesisPrivateKey, err := getEVMSubnetPrefundedKey(
		network,
		flags.SubnetName,
		flags.CChain,
		flags.BlockchainID,
	)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = getPrivateKeyFromFlags(
			deployFlags.PrivateKeyFlags,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
		if privateKey == "" {
			privateKey, err = promptPrivateKey("deploy teleporter", genesisAddress, genesisPrivateKey)
			if err != nil {
				return err
			}
		}
	}
	switch {
	case flags.MessengerContractAddressPath != "" || flags.MessengerDeployerAddressPath != "" || flags.MessengerDeployerTxPath != "" || flags.RegistryBydecodePath != "":
		teleporterVersion = ""
		if flags.MessengerContractAddressPath == "" || flags.MessengerDeployerAddressPath == "" || flags.MessengerDeployerTxPath == "" || flags.RegistryBydecodePath == "" {
			return fmt.Errorf("if setting any teleporter asset path, you must set all teleporter asset paths")
		}
	case flags.Version != "" && flags.Version != "latest":
		teleporterVersion = flags.Version
	case teleporterVersion != "":
	default:
		teleporterInfo, err := teleporter.GetInfo(app)
		if err != nil {
			return err
		}
		teleporterVersion = teleporterInfo.Version
	}
	// deploy to subnet
	rpcURL := network.BlockchainEndpoint(blockchainID)
	if flags.RPCURL != "" {
		rpcURL = flags.RPCURL
	}
	td := teleporter.Deployer{}
	if flags.MessengerContractAddressPath != "" {
		if err := td.SetAssetsFromPaths(
			flags.MessengerContractAddressPath,
			flags.MessengerDeployerAddressPath,
			flags.MessengerDeployerTxPath,
			flags.RegistryBydecodePath,
		); err != nil {
			return err
		}
	} else {
		if err := td.DownloadAssets(
			app.GetTeleporterBinDir(),
			teleporterVersion,
		); err != nil {
			return err
		}
	}
	alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := td.Deploy(
		teleporterSubnetDesc,
		rpcURL,
		privateKey,
		flags.DeployMessenger,
		flags.DeployRegistry,
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
		sc.TeleporterReady = true
		sc.TeleporterVersion = teleporterVersion
		networkInfo := sc.Networks[network.Name()]
		if teleporterMessengerAddress != "" {
			networkInfo.TeleporterMessengerAddress = teleporterMessengerAddress
		}
		if teleporterRegistryAddress != "" {
			networkInfo.TeleporterRegistryAddress = teleporterRegistryAddress
		}
		sc.Networks[network.Name()] = networkInfo
		if err := app.UpdateSidecar(&sc); err != nil {
			return err
		}
	}
	// automatic deploy to cchain for local/devnet
	if !flags.CChain && (network.Kind == models.Local || network.Kind == models.Devnet) {
		ewoq, err := app.GetKey("ewoq", network, false)
		if err != nil {
			return err
		}
		alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := td.Deploy(
			cChainName,
			network.BlockchainEndpoint(cChainAlias),
			ewoq.PrivKeyHex(),
			flags.DeployMessenger,
			flags.DeployRegistry,
		)
		if err != nil {
			return err
		}
		if !alreadyDeployed {
			if network.Kind == models.Local {
				if err := localnet.WriteExtraLocalNetworkData(teleporterMessengerAddress, teleporterRegistryAddress); err != nil {
					return err
				}
			}
			if network.ClusterName != "" {
				clusterConfig, err := app.GetClusterConfig(network.ClusterName)
				if err != nil {
					return err
				}
				if teleporterMessengerAddress != "" {
					clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress = teleporterMessengerAddress
				}
				if teleporterRegistryAddress != "" {
					clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress = teleporterRegistryAddress
				}
				if err := app.SetClusterConfig(network.ClusterName, clusterConfig); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
