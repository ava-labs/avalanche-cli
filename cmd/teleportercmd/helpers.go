// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
)

func getSubnetParams(network models.Network, subnetName string) (ids.ID, ids.ID, string, string, *key.SoftKey, error) {
	var (
		subnetID                   ids.ID
		chainID                    ids.ID
		err                        error
		teleporterMessengerAddress string
		teleporterRegistryAddress  string
		k                          *key.SoftKey
	)
	if isCChain(subnetName) {
		subnetID = ids.Empty
		chainID, err = subnet.GetChainID(network, "C")
		if err != nil {
			return ids.Empty, ids.Empty, "", "", nil, err
		}
		if network.Kind == models.Local {
			extraLocalNetworkData, err := subnet.GetExtraLocalNetworkData(app)
			if err != nil {
				return ids.Empty, ids.Empty, "", "", nil, err
			}
			teleporterMessengerAddress = extraLocalNetworkData.CChainTeleporterMessengerAddress
			teleporterRegistryAddress = extraLocalNetworkData.CChainTeleporterRegistryAddress
			k, err = key.LoadEwoq(network.ID)
			if err != nil {
				return ids.Empty, ids.Empty, "", "", nil, err
			}
		}
	} else {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return ids.Empty, ids.Empty, "", "", nil, err
		}
		subnetID = sc.Networks[network.Name()].SubnetID
		chainID = sc.Networks[network.Name()].BlockchainID
		teleporterMessengerAddress = sc.Networks[network.Name()].TeleporterMessengerAddress
		teleporterRegistryAddress = sc.Networks[network.Name()].TeleporterRegistryAddress
		keyPath := app.GetKeyPath(sc.TeleporterKey)
		k, err = key.LoadSoft(network.ID, keyPath)
		if err != nil {
			return ids.Empty, ids.Empty, "", "", nil, err
		}
	}
	if chainID == ids.Empty {
		return ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("chainID for subnet %s not found on network %s", subnetName, network.Name())
	}
	if teleporterMessengerAddress == "" {
		return ids.Empty, ids.Empty, "", "", nil, fmt.Errorf("teleporter messenger address for subnet %s not found on network %s", subnetName, network.Name())
	}
	return subnetID, chainID, teleporterMessengerAddress, teleporterRegistryAddress, k, nil
}

func isCChain(subnetName string) bool {
	return strings.ToLower(subnetName) == "c-chain" || strings.ToLower(subnetName) == "cchain"
}
