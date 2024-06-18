// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridgecmd

import (
	_ "embed"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"
)

func validateSubnet(network models.Network, subnetName string) error {
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("subnet %s not deployed into %s", subnetName, network.Name())
	}
	return nil
}

func promptChain(
	prompt string,
	network models.Network,
	avoidCChain bool,
	avoidSubnet string,
	chainFlags *ChainFlags,
) (bool, error) {
	subnetNames, err := app.GetSubnetNamesOnNetwork(network)
	if err != nil {
		return false, err
	}
	cancel, _, _, cChain, subnetName, err := prompts.PromptChain(
		app.Prompt,
		prompt,
		subnetNames,
		true,
		true,
		avoidCChain,
		avoidSubnet,
	)
	if err == nil {
		chainFlags.SubnetName = subnetName
		chainFlags.CChain = cChain
	}
	return cancel, err
}

func GetSubnetParams(
	network models.Network,
	subnetName string,
	isCChain bool,
) (string, ids.ID, ids.ID, string, string, *key.SoftKey, error) {
	var (
		subnetID                   ids.ID
		chainID                    ids.ID
		err                        error
		teleporterMessengerAddress string
		teleporterRegistryAddress  string
		k                          *key.SoftKey
		endpoint                   string
	)
	if isCChain {
		subnetID = ids.Empty
		chainID, err = utils.GetChainID(network.Endpoint, "C")
		if err != nil {
			return "", ids.Empty, ids.Empty, "", "", nil, err
		}
		if network.Kind == models.Local {
			b, extraLocalNetworkData, err := subnet.GetExtraLocalNetworkData()
			if err != nil {
				return "", ids.Empty, ids.Empty, "", "", nil, err
			}
			if !b {
				return "", ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("no extra local network data available")
			}
			teleporterMessengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
			teleporterRegistryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
		} else if network.ClusterName != "" {
			clusterConfig, err := app.GetClusterConfig(network.ClusterName)
			if err != nil {
				return "", ids.Empty, ids.Empty, "", "", nil, err
			}
			teleporterMessengerAddress = clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress
			teleporterRegistryAddress = clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress
		}
		k, err = key.LoadEwoq(network.ID)
		if err != nil {
			return "", ids.Empty, ids.Empty, "", "", nil, err
		}
		endpoint = network.CChainEndpoint()
	} else {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return "", ids.Empty, ids.Empty, "", "", nil, err
		}
		if !sc.TeleporterReady {
			return "", ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("subnet %s is not enabled for teleporter", subnetName)
		}
		subnetID = sc.Networks[network.Name()].SubnetID
		chainID = sc.Networks[network.Name()].BlockchainID
		teleporterMessengerAddress = sc.Networks[network.Name()].TeleporterMessengerAddress
		teleporterRegistryAddress = sc.Networks[network.Name()].TeleporterRegistryAddress
		keyPath := app.GetKeyPath(sc.TeleporterKey)
		k, err = key.LoadSoft(network.ID, keyPath)
		if err != nil {
			return "", ids.Empty, ids.Empty, "", "", nil, err
		}
		endpoint = network.BlockchainEndpoint(chainID.String())
	}
	if chainID == ids.Empty {
		return "", ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("chainID for subnet %s not found on network %s", subnetName, network.Name())
	}
	if teleporterMessengerAddress == "" {
		return "", ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("teleporter messenger address for subnet %s not found on network %s", subnetName, network.Name())
	}
	return endpoint, subnetID, chainID, teleporterMessengerAddress, teleporterRegistryAddress, k, nil
}

func getNativeTokenSymbol(subnetName string, isCChain bool) (string, error) {
	nativeTokenSymbol := "AVAX"
	if !isCChain {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return "", err
		}
		nativeTokenSymbol = sc.TokenSymbol
	}
	return nativeTokenSymbol, nil
}
