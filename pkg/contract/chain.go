// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"

	cmdflags "github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/pkg/application"
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

func GetRPCURL(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (string, error) {
	switch {
	case chainSpec.CChain:
		return network.CChainEndpoint(), nil
	case chainSpec.BlockchainID != "":
		return network.BlockchainEndpoint(chainSpec.BlockchainID), nil
	case chainSpec.BlockchainName != "":
		sc, err := app.LoadSidecar(chainSpec.BlockchainName)
		if err != nil {
			return "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return "", fmt.Errorf("blockchain has not been deployed to %s", network.Name())
		}
		return network.BlockchainEndpoint(sc.Networks[network.Name()].BlockchainID.String()), nil
	default:
		return "", fmt.Errorf("blockchain is not defined")
	}
}

func GetBlockchainID(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (string, error) {
	blockchainID := ""
	switch {
	case chainSpec.BlockchainID != "":
		chainID, err := utils.GetChainID(network.Endpoint, chainSpec.BlockchainID)
		if err != nil {
			return "", err
		}
		blockchainID = chainID.String()
	case chainSpec.CChain:
		chainID, err := utils.GetChainID(network.Endpoint, "C")
		if err != nil {
			return "", err
		}
		blockchainID = chainID.String()
	case chainSpec.BlockchainName != "":
		sc, err := app.LoadSidecar(chainSpec.BlockchainName)
		if err != nil {
			return "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return "", fmt.Errorf("blockchain has not been deployed to %s", network.Name())
		}
		blockchainID = sc.Networks[network.Name()].BlockchainID.String()
	default:
		return "", fmt.Errorf("blockchain is not defined")
	}
	return blockchainID, nil
}

func GetSubnetID(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (string, error) {
	subnetID := ""
	switch {
	case chainSpec.CChain:
		subnetID = ids.Empty.String()
	case chainSpec.BlockchainName != "":
		sc, err := app.LoadSidecar(chainSpec.BlockchainName)
		if err != nil {
			return "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return "", fmt.Errorf("blockchain has not been deployed to %s", network.Name())
		}
		subnetID = sc.Networks[network.Name()].SubnetID.String()
	case chainSpec.BlockchainID != "":
		blockchainID, err := ids.FromString(chainSpec.BlockchainID)
		if err != nil {
			return "", fmt.Errorf("failure parsing %s as id: %w", chainSpec.BlockchainID, err)
		}
		tx, err := utils.GetBlockchainTx(network.Endpoint, blockchainID)
		if err != nil {
			return "", err
		}
		subnetID = tx.SubnetID.String()
	default:
		return "", fmt.Errorf("blockchain is not defined")
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
) (string, string, error) {
	messengerAddress := ""
	registryAddress := ""
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
		messengerAddress = sc.Networks[network.Name()].TeleporterMessengerAddress
		registryAddress = sc.Networks[network.Name()].TeleporterRegistryAddress
	default:
		return "", "", fmt.Errorf("blockchain is not defined")
	}
	blockchainDesc, err := GetBlockchainDesc(chainSpec)
	if err != nil {
		return "", "", err
	}
	if registryAddress == "" {
		addr, err := app.Prompt.CaptureAddress("Which is the ICM Registry address for " + blockchainDesc)
		if err != nil {
			return "", "", err
		}
		registryAddress = addr.Hex()
	}
	if messengerAddress == "" {
		addr, err := app.Prompt.CaptureAddress("Which is the ICM Messenger address for " + blockchainDesc)
		if err != nil {
			return "", "", err
		}
		messengerAddress = addr.Hex()
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
	if network.Kind == models.Local {
		b, extraLocalNetworkData, err := localnet.GetExtraLocalNetworkData()
		if err != nil {
			return "", "", err
		}
		if !b {
			return "", "", fmt.Errorf("no extra local network data available")
		}
		messengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
		registryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
	} else if network.ClusterName != "" {
		clusterConfig, err := app.GetClusterConfig(network.ClusterName)
		if err != nil {
			return "", "", err
		}
		messengerAddress = clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress
		registryAddress = clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress
	}
	return registryAddress, messengerAddress, nil
}
