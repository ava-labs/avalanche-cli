// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

type ChainSpec struct {
	BlockchainName string
	CChain         bool
	BlockchainID   string
}

func MutuallyExclusiveChainSpecFields(chainSpec ChainSpec) bool {
	return cmdflags.EnsureMutuallyExclusive([]bool{
		chainSpec.BlockchainName != "",
		chainSpec.BlockchainID != "",
		chainSpec.CChain,
	})
}

func DefinedChainSpec(chainSpec ChainSpec) bool {
	return chainSpec.BlockchainName != "" || chainSpec.BlockchainID != "" || chainSpec.CChain
}

func AddChainSpecToCmd(
	cmd *cobra.Command,
	chainSpec *ChainSpec,
	goal string,
	blockchainFlagName string,
	cChainFlagName string,
	blockchainIDFlagName string,
	addBlockchainIDFlag bool,
) {
	subnetFlagName := "" // backwards compat
	if blockchainFlagName == "" {
		blockchainFlagName = "blockchain"
		subnetFlagName = "subnet"
	}
	if cChainFlagName == "" {
		cChainFlagName = "c-chain"
	}
	if blockchainIDFlagName == "" {
		blockchainIDFlagName = "blockchain-id"
	}
	if subnetFlagName != "" {
		cmd.Flags().StringVar(&chainSpec.BlockchainName, subnetFlagName, "", fmt.Sprintf("%s into the given CLI blockchain", goal))
	}
	cmd.Flags().StringVar(&chainSpec.BlockchainName, blockchainFlagName, "", fmt.Sprintf("%s into the given CLI blockchain", goal))
	cmd.Flags().BoolVar(&chainSpec.CChain, cChainFlagName, false, fmt.Sprintf("%s into C-Chain", goal))
	if addBlockchainIDFlag {
		cmd.Flags().StringVar(&chainSpec.BlockchainID, blockchainIDFlagName, "", fmt.Sprintf("%s into the given blockchain ID/Alias", goal))
	}
}

func GetBlockchainEndpoints(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
	promptForRPCEndpoint bool,
	promptForWSEndpoint bool,
) (string, string, error) {
	var (
		rpcEndpoint string
		wsEndpoint  string
	)
	switch {
	case chainSpec.CChain:
		rpcEndpoint = network.CChainEndpoint()
		wsEndpoint = network.CChainWSEndpoint()
	case network.Kind == models.Local || network.Kind == models.Devnet:
		blockchainID, err := GetBlockchainID(app, network, chainSpec)
		if err != nil {
			return "", "", err
		}
		rpcEndpoint = network.BlockchainEndpoint(blockchainID.String())
		wsEndpoint = network.BlockchainWSEndpoint(blockchainID.String())
	case chainSpec.BlockchainName != "":
		sc, err := app.LoadSidecar(chainSpec.BlockchainName)
		if err != nil {
			return "", "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return "", "", fmt.Errorf("blockchain has not been deployed to %s", network.Name())
		}
		rpcEndpoint = sc.Networks[network.Name()].RPCEndpoint
		wsEndpoint = sc.Networks[network.Name()].WSEndpoint
	}
	blockchainDesc, err := GetBlockchainDesc(chainSpec)
	if err != nil {
		return "", "", err
	}
	if rpcEndpoint == "" && promptForRPCEndpoint {
		rpcEndpoint, err = app.Prompt.CaptureURL("Which is the RPC endpoint for "+blockchainDesc, false)
		if err != nil {
			return "", "", err
		}
	}
	if wsEndpoint == "" && promptForWSEndpoint {
		wsEndpoint, err = app.Prompt.CaptureURL("Which is the WS endpoint for "+blockchainDesc, false)
		if err != nil {
			return "", "", err
		}
	}
	return rpcEndpoint, wsEndpoint, nil
}

func GetBlockchainID(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (ids.ID, error) {
	var blockchainID ids.ID
	switch {
	case chainSpec.BlockchainID != "":
		chainID, err := utils.GetChainID(network.Endpoint, chainSpec.BlockchainID)
		if err != nil {
			return ids.Empty, err
		}
		blockchainID = chainID
	case chainSpec.CChain:
		chainID, err := utils.GetChainID(network.Endpoint, "C")
		if err != nil {
			return ids.Empty, err
		}
		blockchainID = chainID
	case chainSpec.BlockchainName != "":
		sc, err := app.LoadSidecar(chainSpec.BlockchainName)
		if err != nil {
			return ids.Empty, fmt.Errorf("failed to load sidecar: %w", err)
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return ids.Empty, fmt.Errorf("blockchain has not been deployed to %s", network.Name())
		}
		blockchainID = sc.Networks[network.Name()].BlockchainID
	default:
		return ids.Empty, fmt.Errorf("blockchain is not defined")
	}
	return blockchainID, nil
}

func GetSubnetID(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (ids.ID, error) {
	var subnetID ids.ID
	switch {
	case chainSpec.CChain:
		subnetID = ids.Empty
	case chainSpec.BlockchainName != "":
		sc, err := app.LoadSidecar(chainSpec.BlockchainName)
		if err != nil {
			return ids.Empty, fmt.Errorf("failed to load sidecar: %w", err)
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return ids.Empty, fmt.Errorf("blockchain has not been deployed to %s", network.Name())
		}
		subnetID = sc.Networks[network.Name()].SubnetID
	case chainSpec.BlockchainID != "":
		blockchainID, err := ids.FromString(chainSpec.BlockchainID)
		if err != nil {
			return ids.Empty, fmt.Errorf("failure parsing %s as id: %w", chainSpec.BlockchainID, err)
		}
		tx, err := utils.GetBlockchainTx(network.Endpoint, blockchainID)
		if err != nil {
			return ids.Empty, err
		}
		subnetID = tx.SubnetID
	default:
		return ids.Empty, fmt.Errorf("blockchain is not defined")
	}
	return subnetID, nil
}

func GetBlockchainDesc(
	chainSpec ChainSpec,
) (string, error) {
	blockchainDesc := ""
	switch {
	case chainSpec.BlockchainName != "":
		blockchainDesc = chainSpec.BlockchainName
	case chainSpec.CChain:
		blockchainDesc = "C-Chain"
	case chainSpec.BlockchainID != "":
		blockchainDesc = chainSpec.BlockchainID
	default:
		return "", fmt.Errorf("blockchain is not defined")
	}
	return blockchainDesc, nil
}

func GetICMInfo(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
	promptForRegistry bool,
	promptForMessenger bool,
	defaultToLatestReleasedMessenger bool,
) (string, string, error) {
	registryAddress := ""
	messengerAddress := ""
	switch {
	case chainSpec.CChain:
		var err error
		registryAddress, messengerAddress, err = GetCChainICMInfo(app, network)
		if err != nil {
			return "", "", err
		}
	case chainSpec.BlockchainID != "":
	case chainSpec.BlockchainName != "":
		sc, err := app.LoadSidecar(chainSpec.BlockchainName)
		if err != nil {
			return "", "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return "", "", fmt.Errorf("blockchain has not been deployed to %s", network.Name())
		}
		registryAddress = sc.Networks[network.Name()].TeleporterRegistryAddress
		messengerAddress = sc.Networks[network.Name()].TeleporterMessengerAddress
	default:
		return "", "", fmt.Errorf("blockchain is not defined")
	}
	blockchainDesc, err := GetBlockchainDesc(chainSpec)
	if err != nil {
		return "", "", err
	}
	if registryAddress == "" && promptForRegistry {
		addr, err := app.Prompt.CaptureAddress("Which is the ICM Registry address for " + blockchainDesc)
		if err != nil {
			return "", "", err
		}
		registryAddress = addr.Hex()
	}
	if messengerAddress == "" {
		if promptForMessenger {
			addr, err := app.Prompt.CaptureAddress("Which is the ICM Messenger address for " + blockchainDesc)
			if err != nil {
				return "", "", err
			}
			messengerAddress = addr.Hex()
		} else if defaultToLatestReleasedMessenger {
			messengerAddress = constants.DefaultTeleporterMessengerAddress
		}
	}
	return registryAddress, messengerAddress, nil
}

func PromptChain(
	app *application.Avalanche,
	network models.Network,
	prompt string,
	avoidCChain bool,
	avoidBlockchain string,
	includeCustom bool,
	chainSpec *ChainSpec,
) (bool, error) {
	blockchainNames, err := app.GetBlockchainNamesOnNetwork(network)
	if err != nil {
		return false, err
	}
	cancel, _, _, cChain, blockchainName, blockchainID, err := prompts.PromptChain(
		app.Prompt,
		prompt,
		blockchainNames,
		true,
		true,
		avoidCChain,
		avoidBlockchain,
		includeCustom,
	)
	if err != nil || cancel {
		return cancel, err
	}
	if blockchainID != "" {
		// map from alias to blockchain ID (or identity)
		chainID, err := utils.GetChainID(network.Endpoint, blockchainID)
		if err != nil {
			return cancel, err
		}
		blockchainID = chainID.String()
	}
	chainSpec.BlockchainName = blockchainName
	chainSpec.CChain = cChain
	chainSpec.BlockchainID = blockchainID
	return false, nil
}

func GetCChainICMInfo(
	app *application.Avalanche,
	network models.Network,
) (string, string, error) {
	messengerAddress := ""
	registryAddress := ""
	switch {
	case network.Kind == models.Local:
		b, extraLocalNetworkData, err := localnet.GetExtraLocalNetworkData()
		if err != nil {
			return "", "", err
		}
		if !b {
			return "", "", fmt.Errorf("no extra local network data available")
		}
		messengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
		registryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
	case network.ClusterName != "":
		clusterConfig, err := app.GetClusterConfig(network.ClusterName)
		if err != nil {
			return "", "", err
		}
		messengerAddress = clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress
		registryAddress = clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress
	case network.Kind == models.Fuji:
		messengerAddress = constants.DefaultTeleporterMessengerAddress
		registryAddress = constants.FujiCChainTeleporterRegistryAddress
	case network.Kind == models.Mainnet:
		messengerAddress = constants.DefaultTeleporterMessengerAddress
		registryAddress = constants.MainnetCChainTeleporterRegistryAddress
	}
	return registryAddress, messengerAddress, nil
}
