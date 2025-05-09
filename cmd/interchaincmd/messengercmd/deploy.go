// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package messengercmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/spf13/cobra"
)

type DeployFlags struct {
	Network                      networkoptions.NetworkFlags
	ChainFlags                   contract.ChainSpec
	KeyName                      string
	GenesisKey                   bool
	DeployMessenger              bool
	DeployRegistry               bool
	ForceRegistryDeploy          bool
	RPCURL                       string
	Version                      string
	MessengerContractAddressPath string
	MessengerDeployerAddressPath string
	MessengerDeployerTxPath      string
	RegistryBydecodePath         string
	PrivateKeyFlags              contract.PrivateKeyFlags
	IncludeCChain                bool
	CChainKeyName                string
}

const (
	cChainAlias = "C"
	cChainName  = "c-chain"
)

var deployFlags DeployFlags

// avalanche interchain messenger deploy
func NewDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys ICM Messenger and Registry into a given L1",
		Long:  `Deploys ICM Messenger and Registry into a given L1.

For Local Networks, it also deploys into C-Chain.`,
		RunE:  deploy,
		Args:  cobrautils.ExactArgs(0),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &deployFlags.Network, true, networkoptions.DefaultSupportedNetworkOptions)
	deployFlags.PrivateKeyFlags.AddToCmd(cmd, "to fund ICM deploy")
	deployFlags.ChainFlags.SetEnabled(true, true, false, false, true)
	deployFlags.ChainFlags.AddToCmd(cmd, "deploy ICM into %s")
	cmd.Flags().BoolVar(&deployFlags.DeployMessenger, "deploy-messenger", true, "deploy ICM Messenger")
	cmd.Flags().BoolVar(&deployFlags.DeployRegistry, "deploy-registry", true, "deploy ICM Registry")
	cmd.Flags().BoolVar(&deployFlags.ForceRegistryDeploy, "force-registry-deploy", false, "deploy ICM Registry even if Messenger has already been deployed")
	cmd.Flags().StringVar(&deployFlags.RPCURL, "rpc-url", "", "use the given RPC URL to connect to the subnet")
	cmd.Flags().StringVar(&deployFlags.Version, "version", "latest", "version to deploy")
	cmd.Flags().StringVar(&deployFlags.MessengerContractAddressPath, "messenger-contract-address-path", "", "path to a messenger contract address file")
	cmd.Flags().StringVar(&deployFlags.MessengerDeployerAddressPath, "messenger-deployer-address-path", "", "path to a messenger deployer address file")
	cmd.Flags().StringVar(&deployFlags.MessengerDeployerTxPath, "messenger-deployer-tx-path", "", "path to a messenger deployer tx file")
	cmd.Flags().StringVar(&deployFlags.RegistryBydecodePath, "registry-bytecode-path", "", "path to a registry bytecode file")
	cmd.Flags().BoolVar(&deployFlags.IncludeCChain, "include-cchain", false, "deploy ICM also to C-Chain")
	cmd.Flags().StringVar(&deployFlags.CChainKeyName, "cchain-key", "", "key to be used to pay fees to deploy ICM to C-Chain")
	return cmd
}

func deploy(_ *cobra.Command, args []string) error {
	return CallDeploy(args, deployFlags, models.UndefinedNetwork)
}

func CallDeploy(_ []string, flags DeployFlags, network models.Network) error {
	var err error
	if network == models.UndefinedNetwork {
		network, err = networkoptions.GetNetworkFromCmdLineFlags(
			app,
			"On what Network do you want to deploy the ICM Messenger?",
			flags.Network,
			true,
			false,
			networkoptions.DefaultSupportedNetworkOptions,
			"",
		)
		if err != nil {
			return err
		}
	}
	if err := flags.ChainFlags.CheckMutuallyExclusiveFields(); err != nil {
		return err
	}
	if !flags.DeployMessenger && !flags.DeployRegistry {
		return fmt.Errorf("you should set at least one of --deploy-messenger/--deploy-registry to true")
	}
	if !flags.ChainFlags.Defined() {
		prompt := "Which Blockchain would you like to deploy ICM to?"
		if cancel, err := contract.PromptChain(
			app,
			network,
			prompt,
			"",
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
			"deploy ICM",
			app.GetKeyDir(),
			app.GetKey,
			genesisAddress,
			genesisPrivateKey,
		)
		if err != nil {
			return err
		}
	}
	var icmVersion string
	switch {
	case flags.MessengerContractAddressPath != "" || flags.MessengerDeployerAddressPath != "" || flags.MessengerDeployerTxPath != "" || flags.RegistryBydecodePath != "":
		if flags.MessengerContractAddressPath == "" || flags.MessengerDeployerAddressPath == "" || flags.MessengerDeployerTxPath == "" || flags.RegistryBydecodePath == "" {
			return fmt.Errorf("if setting any ICM asset path, you must set all ICM asset paths")
		}
	case flags.Version != "" && flags.Version != "latest":
		icmVersion = flags.Version
	default:
		icmInfo, err := interchain.GetICMInfo(app)
		if err != nil {
			return err
		}
		icmVersion = icmInfo.Version
	}
	// deploy to subnet
	td := interchain.ICMDeployer{}
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
			app.GetICMContractsBinDir(),
			icmVersion,
		); err != nil {
			return err
		}
	}
	blockchainDesc, err := contract.GetBlockchainDesc(flags.ChainFlags)
	if err != nil {
		return err
	}
	alreadyDeployed, messengerAddress, registryAddress, err := td.Deploy(
		blockchainDesc,
		rpcURL,
		privateKey,
		flags.DeployMessenger,
		flags.DeployRegistry,
		flags.ForceRegistryDeploy,
	)
	if err != nil {
		return err
	}
	if flags.ChainFlags.BlockchainName != "" && (!alreadyDeployed || flags.ForceRegistryDeploy) {
		// update sidecar
		sc, err := app.LoadSidecar(flags.ChainFlags.BlockchainName)
		if err != nil {
			return fmt.Errorf("failed to load sidecar: %w", err)
		}
		sc.TeleporterReady = true
		sc.TeleporterVersion = icmVersion
		networkInfo := sc.Networks[network.Name()]
		if messengerAddress != "" {
			networkInfo.TeleporterMessengerAddress = messengerAddress
		}
		if registryAddress != "" {
			networkInfo.TeleporterRegistryAddress = registryAddress
		}
		sc.Networks[network.Name()] = networkInfo
		if err := app.UpdateSidecar(&sc); err != nil {
			return err
		}
	}
	// automatic deploy to cchain for local
	if !flags.ChainFlags.CChain && (network.Kind == models.Local || flags.IncludeCChain) {
		if flags.CChainKeyName == "" {
			flags.CChainKeyName = "ewoq"
		}
		ewoq, err := app.GetKey(flags.CChainKeyName, network, false)
		if err != nil {
			return err
		}
		alreadyDeployed, messengerAddress, registryAddress, err := td.Deploy(
			cChainName,
			network.BlockchainEndpoint(cChainAlias),
			ewoq.PrivKeyHex(),
			flags.DeployMessenger,
			flags.DeployRegistry,
			false,
		)
		if err != nil {
			return err
		}
		if !alreadyDeployed {
			if network.Kind == models.Local {
				if err := localnet.WriteExtraLocalNetworkData(
					app,
					"",
					"",
					messengerAddress,
					registryAddress,
				); err != nil {
					return err
				}
			}
			if network.ClusterName != "" {
				clusterConfig, err := app.GetClusterConfig(network.ClusterName)
				if err != nil {
					return err
				}
				if messengerAddress != "" {
					clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress = messengerAddress
				}
				if registryAddress != "" {
					clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress = registryAddress
				}
				if err := app.SetClusterConfig(network.ClusterName, clusterConfig); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
