// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleporter

import (
	_ "embed"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"
)

// For the given network and chain name, return parameters commonly used in interchain apps:
// - url endpoint
// - subnet ID
// - chain ID
// - messenger address
// - registry address
// - preconfigured key for interchain
func GetBlockchainParams(
	app *application.Avalanche,
	network models.Network,
	blockchainName string,
	isCChain bool,
) (string, string, ids.ID, ids.ID, string, string, *key.SoftKey, error) {
	var (
		subnetID                   ids.ID
		chainID                    ids.ID
		err                        error
		teleporterMessengerAddress string
		teleporterRegistryAddress  string
		k                          *key.SoftKey
		endpoint                   string
		name                       string
	)
	if isCChain {
		subnetID = ids.Empty
		chainID, err = utils.GetChainID(network.Endpoint, "C")
		if err != nil {
			return "", "", ids.Empty, ids.Empty, "", "", nil, err
		}
		if network.Kind == models.Local {
			b, extraLocalNetworkData, err := localnet.GetExtraLocalNetworkData()
			if err != nil {
				return "", "", ids.Empty, ids.Empty, "", "", nil, err
			}
			if !b {
				return "", "", ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("no extra local network data available")
			}
			teleporterMessengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
			teleporterRegistryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
		} else if network.ClusterName != "" {
			clusterConfig, err := app.GetClusterConfig(network.ClusterName)
			if err != nil {
				return "", "", ids.Empty, ids.Empty, "", "", nil, err
			}
			teleporterMessengerAddress = clusterConfig.ExtraNetworkData.CChainTeleporterMessengerAddress
			teleporterRegistryAddress = clusterConfig.ExtraNetworkData.CChainTeleporterRegistryAddress
		}
		k, err = key.LoadEwoq(network.ID)
		if err != nil {
			return "", "", ids.Empty, ids.Empty, "", "", nil, err
		}
		endpoint = network.CChainEndpoint()
		name = "C-Chain"
	} else {
		sc, err := app.LoadSidecar(blockchainName)
		if err != nil {
			return "", "", ids.Empty, ids.Empty, "", "", nil, err
		}
		if !sc.TeleporterReady {
			return "", "", ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("subnet %s is not enabled for teleporter", blockchainName)
		}
		subnetID = sc.Networks[network.Name()].SubnetID
		chainID = sc.Networks[network.Name()].BlockchainID
		teleporterMessengerAddress = sc.Networks[network.Name()].TeleporterMessengerAddress
		teleporterRegistryAddress = sc.Networks[network.Name()].TeleporterRegistryAddress
		keyPath := app.GetKeyPath(sc.TeleporterKey)
		k, err = key.LoadSoft(network.ID, keyPath)
		if err != nil {
			return "", "", ids.Empty, ids.Empty, "", "", nil, err
		}
		endpoint = network.BlockchainEndpoint(chainID.String())
		name = blockchainName
	}
	if chainID == ids.Empty {
		return "", "", ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("chainID for subnet %s not found on network %s", blockchainName, network.Name())
	}
	if teleporterMessengerAddress == "" {
		return "", "", ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("teleporter messenger address for subnet %s not found on network %s", blockchainName, network.Name())
	}
	return endpoint, name, subnetID, chainID, teleporterMessengerAddress, teleporterRegistryAddress, k, nil
}
