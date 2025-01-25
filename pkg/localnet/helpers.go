// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"encoding/json"
	"path/filepath"

	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"
)

func GetPerNodeBlockchainConfig(
	app *application.Avalanche,
	blockchainName string,
) (map[ids.NodeID][]byte, error) {
	perNodeBlockchainConfig := map[ids.NodeID][]byte{}
	path := filepath.Join(app.GetSubnetDir(), blockchainName, constants.PerNodeChainConfigFileName)
	if utils.FileExists(path) {
		config, err := utils.ReadJSON(path)
		if err != nil {
			return nil, err
		}
		for nodeIDStr, nodeBlockchainConfig := range config {
			bs, err := json.Marshal(nodeBlockchainConfig)
			if err != nil {
				return nil, err
			}
			nodeID, err := ids.NodeIDFromString(nodeIDStr)
			if err != nil {
				return nil, err
			}
			perNodeBlockchainConfig[nodeID] = bs
		}
	}
	return perNodeBlockchainConfig, nil
}

func PersistDefaultBlockchainEndpoints(
	app *application.Avalanche,
	networkModel models.Network,
	nodeURIs []string,
	blockchainName string,
) error {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	networkInfo := sc.Networks[networkModel.Name()]
	if networkInfo.BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, networkModel.Name())
	}
	rpcEndpoints := set.Of(networkInfo.RPCEndpoints...)
	wsEndpoints := set.Of(networkInfo.WSEndpoints...)
	for _, nodeURI := range nodeURIs {
		rpcEndpoints.Add(models.GetRPCEndpoint(nodeURI, networkInfo.BlockchainID.String()))
		wsEndpoints.Add(models.GetWSEndpoint(nodeURI, networkInfo.BlockchainID.String()))
	}
	networkInfo.RPCEndpoints = rpcEndpoints.List()
	networkInfo.WSEndpoints = wsEndpoints.List()
	sc.Networks[networkModel.Name()] = networkInfo
	return app.UpdateSidecar(&sc)
}
