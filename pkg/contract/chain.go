// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"
	"strings"

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
	BlockchainName            string
	blockchainNameFlagEnabled bool
	blockchainNameFlagName    string
	CChain                    bool
	cChainFlagEnabled         bool
	cChainFlagName            string
	PChain                    bool
	pChainFlagEnabled         bool
	pChainFlagName            string
	XChain                    bool
	xChainFlagEnabled         bool
	xChainFlagName            string
	BlockchainID              string
	blockchainIDFlagEnabled   bool
	blockchainIDFlagName      string
	OnlySOV                   bool
}

const (
	defaultBlockchainNameFlagName = "blockchain"
	defaultCChainFlagName         = "c-chain"
	defaultPChainFlagName         = "p-chain"
	defaultXChainFlagName         = "x-chain"
	defaultBlockchainIDFlagName   = "blockchain-id"
)

func (cs *ChainSpec) CheckMutuallyExclusiveFields() error {
	if !cmdflags.EnsureMutuallyExclusive([]bool{
		cs.BlockchainName != "",
		cs.BlockchainID != "",
		cs.CChain,
		cs.PChain,
		cs.XChain,
	}) {
		flags := []string{}
		if cs.blockchainNameFlagEnabled {
			flags = append(flags, cs.blockchainNameFlagName)
		}
		if cs.cChainFlagEnabled {
			flags = append(flags, cs.cChainFlagName)
		}
		if cs.pChainFlagEnabled {
			flags = append(flags, cs.pChainFlagName)
		}
		if cs.xChainFlagEnabled {
			flags = append(flags, cs.xChainFlagName)
		}
		if cs.blockchainIDFlagEnabled {
			flags = append(flags, cs.blockchainIDFlagName)
		}
		return fmt.Errorf("%s are mutually exclusive flags",
			strings.Join(flags, ", "),
		)
	}
	return nil
}

func (cs *ChainSpec) Defined() bool {
	if cs.blockchainNameFlagEnabled && cs.BlockchainName != "" {
		return true
	}
	if cs.cChainFlagEnabled && cs.CChain {
		return true
	}
	if cs.pChainFlagEnabled && cs.PChain {
		return true
	}
	if cs.xChainFlagEnabled && cs.XChain {
		return true
	}
	if cs.blockchainIDFlagEnabled && cs.BlockchainID != "" {
		return true
	}
	return false
}

func (cs *ChainSpec) fillDefaults() {
	if cs.blockchainNameFlagName == "" {
		cs.blockchainIDFlagEnabled = true
		cs.blockchainNameFlagName = defaultBlockchainNameFlagName
	}
	if cs.cChainFlagName == "" {
		cs.cChainFlagEnabled = true
		cs.cChainFlagName = defaultCChainFlagName
	}
	if cs.pChainFlagName == "" {
		cs.pChainFlagName = defaultPChainFlagName
	}
	if cs.xChainFlagName == "" {
		cs.xChainFlagName = defaultXChainFlagName
	}
	if cs.blockchainIDFlagName == "" {
		cs.blockchainIDFlagName = defaultBlockchainIDFlagName
	}
}

func (cs *ChainSpec) SetFlagNames(
	blockchainNameFlagName string,
	cChainFlagName string,
	pChainFlagName string,
	xChainFlagName string,
	blockchainIDFlagName string,
) {
	cs.blockchainNameFlagName = blockchainNameFlagName
	cs.cChainFlagName = cChainFlagName
	cs.pChainFlagName = pChainFlagName
	cs.xChainFlagName = xChainFlagName
	cs.blockchainIDFlagName = blockchainIDFlagName
	cs.SetEnabled(
		cs.blockchainNameFlagName != "",
		cs.cChainFlagName != "",
		cs.pChainFlagName != "",
		cs.xChainFlagName != "",
		cs.blockchainIDFlagName != "",
	)
}

func (cs *ChainSpec) SetEnabled(
	blockchainNameFlagEnabled bool,
	cChainFlagEnabled bool,
	pChainFlagEnabled bool,
	xChainFlagEnabled bool,
	blockchainIDFlagEnabled bool,
) {
	cs.blockchainNameFlagEnabled = blockchainNameFlagEnabled
	cs.cChainFlagEnabled = cChainFlagEnabled
	cs.pChainFlagEnabled = pChainFlagEnabled
	cs.xChainFlagEnabled = xChainFlagEnabled
	cs.blockchainIDFlagEnabled = blockchainIDFlagEnabled
}

func (cs *ChainSpec) AddToCmd(
	cmd *cobra.Command,
	goalFmt string,
) {
	cs.fillDefaults()
	if cs.blockchainNameFlagEnabled {
		cmd.Flags().StringVar(&cs.BlockchainName, cs.blockchainNameFlagName, "", fmt.Sprintf(goalFmt, "the given CLI blockchain"))
	}
	if cs.cChainFlagEnabled {
		cmd.Flags().BoolVar(&cs.CChain, cs.cChainFlagName, false, fmt.Sprintf(goalFmt, "C-Chain"))
	}
	if cs.pChainFlagEnabled {
		cmd.Flags().BoolVar(&cs.PChain, cs.pChainFlagName, false, fmt.Sprintf(goalFmt, "P-Chain"))
	}
	if cs.xChainFlagEnabled {
		cmd.Flags().BoolVar(&cs.XChain, cs.xChainFlagName, false, fmt.Sprintf(goalFmt, "X-Chain"))
	}
	if cs.blockchainIDFlagEnabled {
		cmd.Flags().StringVar(&cs.BlockchainID, cs.blockchainIDFlagName, "", fmt.Sprintf(goalFmt, "the given blockchain ID/Alias"))
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
	if chainSpec.BlockchainName != "" {
		sc, err := app.LoadSidecar(chainSpec.BlockchainName)
		if err != nil {
			return "", "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		networkName := network.Name()
		if sc.Networks[networkName].BlockchainID == ids.Empty {
			// look into the cluster deploys
			for k := range sc.Networks {
				sidecarNetwork, err := app.GetNetworkFromSidecarNetworkName(k)
				if err == nil {
					if sidecarNetwork.Equals(network) {
						networkName = sidecarNetwork.Name()
					}
				}
			}
		}
		if len(sc.Networks[networkName].RPCEndpoints) > 0 {
			rpcEndpoint = sc.Networks[networkName].RPCEndpoints[0]
		}
		if len(sc.Networks[networkName].WSEndpoints) > 0 {
			wsEndpoint = sc.Networks[networkName].WSEndpoints[0]
		}
	}
	if rpcEndpoint == "" && chainSpec.CChain {
		rpcEndpoint = network.CChainEndpoint()
		wsEndpoint = network.CChainWSEndpoint()
	}
	blockchainDesc, err := GetBlockchainDesc(chainSpec)
	if err != nil {
		return "", "", err
	}
	if rpcEndpoint == "" && promptForRPCEndpoint {
		rpcEndpoint, err = app.Prompt.CaptureURL("What is the RPC endpoint for "+blockchainDesc, false)
		if err != nil {
			return "", "", err
		}
	}
	if wsEndpoint == "" && promptForWSEndpoint {
		wsEndpoint, err = app.Prompt.CaptureURL("What is the WS endpoint for "+blockchainDesc, false)
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
		var err error
		blockchainID, err = ids.FromString(chainSpec.BlockchainID)
		if err != nil {
			// it should be an alias at this point
			blockchainID, err = utils.GetChainID(network.Endpoint, chainSpec.BlockchainID)
			if err != nil {
				return ids.Empty, err
			}
		}
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
	case chainSpec.PChain:
		blockchainDesc = "P-Chain"
	case chainSpec.XChain:
		blockchainDesc = "X-Chain"
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
			messengerAddress = constants.DefaultICMMessengerAddress
		}
	}
	return registryAddress, messengerAddress, nil
}

func PromptChain(
	app *application.Avalanche,
	network models.Network,
	prompt string,
	blockchainNameToAvoid string,
	chainSpec *ChainSpec,
) (bool, error) {
	var (
		err             error
		blockchainNames []string
	)
	if chainSpec.blockchainNameFlagEnabled {
		blockchainNames, err = app.GetBlockchainNamesOnNetwork(network, chainSpec.OnlySOV)
		if err != nil {
			return false, err
		}
	}
	cancel, pChain, xChain, cChain, blockchainName, blockchainID, err := prompts.PromptChain(
		app.Prompt,
		prompt,
		blockchainNames,
		chainSpec.pChainFlagEnabled,
		chainSpec.xChainFlagEnabled,
		chainSpec.cChainFlagEnabled,
		blockchainNameToAvoid,
		chainSpec.blockchainIDFlagEnabled,
	)
	if err != nil || cancel {
		return cancel, err
	}
	if blockchainID != "" {
		chainID, err := ids.FromString(blockchainID)
		if err != nil {
			// map from alias to blockchain ID (or identity)
			chainID, err = utils.GetChainID(network.Endpoint, blockchainID)
			if err != nil {
				return cancel, err
			}
		}
		blockchainID = chainID.String()
	}
	chainSpec.BlockchainName = blockchainName
	chainSpec.CChain = cChain
	chainSpec.PChain = pChain
	chainSpec.XChain = xChain
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
		b, extraLocalNetworkData, err := localnet.GetExtraLocalNetworkData(app, "")
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
		messengerAddress = constants.DefaultICMMessengerAddress
		registryAddress = constants.FujiCChainICMRegistryAddress
	case network.Kind == models.Mainnet:
		messengerAddress = constants.DefaultICMMessengerAddress
		registryAddress = constants.MainnetCChainICMRegistryAddress
	}
	return registryAddress, messengerAddress, nil
}
