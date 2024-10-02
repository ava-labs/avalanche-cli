// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/utils/logging"
)

type DeployFlags struct {
	Network                      networkoptions.NetworkFlags
	ChainFlags                   contract.ChainSpec
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
	PrivateKeyFlags              contract.PrivateKeyFlags
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
	deployFlags.PrivateKeyFlags.AddToCmd(cmd, "to fund ICM deploy")
	deployFlags.ChainFlags.AddToCmd(cmd, "deploy ICM", true)
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
	if err := flags.ChainFlags.CheckMutuallyExclusiveFields(); err != nil {
		return err
	}
	if !flags.DeployMessenger && !flags.DeployRegistry {
		return errors.New("you should set at least one of --deploy-messenger/--deploy-registry to true")
	}
	if !flags.ChainFlags.Defined() {
		prompt := "Which Blockchain would you like to deploy Teleporter to?"
		if cancel, err := contract.PromptChain(
			app,
			network,
			prompt,
			false,
			"",
			false,
			&flags.ChainFlags,
		); err != nil {
			return err
		} else if cancel {
			return nil
		}
	}
	rpcURL := flags.RPCURL
	if rpcURL == "" {
		rpcURL, _, err = contract.GetBlockchainEndpoints(app, network, flags.ChainFlags, true, false)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap("RPC Endpoint: %s"), rpcURL)
	}

	genesisAddress, genesisPrivateKey, err := contract.GetEVMSubnetPrefundedKey(
		app,
		network,
		flags.ChainFlags,
	)
	if err != nil {
		return err
	}
	privateKey, err := flags.PrivateKeyFlags.GetPrivateKey(app, genesisPrivateKey)
	if err != nil {
		return err
	}
	if privateKey == "" {
		privateKey, err = prompts.PromptPrivateKey(
			app.Prompt,
			"deploy teleporter",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}
	var teleporterVersion string
	switch {
	case flags.MessengerContractAddressPath != "" || flags.MessengerDeployerAddressPath != "" || flags.MessengerDeployerTxPath != "" || flags.RegistryBydecodePath != "":
		if flags.MessengerContractAddressPath == "" || flags.MessengerDeployerAddressPath == "" || flags.MessengerDeployerTxPath == "" || flags.RegistryBydecodePath == "" {
			return errors.New("if setting any teleporter asset path, you must set all teleporter asset paths")
		}
	case flags.Version != "" && flags.Version != "latest":
		teleporterVersion = flags.Version
	default:
		teleporterInfo, err := teleporter.GetInfo(app)
		if err != nil {
			return err
		}
		teleporterVersion = teleporterInfo.Version
	}
	// deploy to subnet
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
	blockchainDesc, err := contract.GetBlockchainDesc(flags.ChainFlags)
	if err != nil {
		return err
	}
	alreadyDeployed, teleporterMessengerAddress, teleporterRegistryAddress, err := td.Deploy(
		blockchainDesc,
		rpcURL,
		privateKey,
		flags.DeployMessenger,
		flags.DeployRegistry,
	)
	if err != nil {
		return err
	}
	if flags.ChainFlags.BlockchainName != "" && !alreadyDeployed {
		// update sidecar
		sc, err := app.LoadSidecar(flags.ChainFlags.BlockchainName)
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
	if !flags.ChainFlags.CChain && (network.Kind == models.Local || network.Kind == models.Devnet) {
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
