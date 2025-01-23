// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"

	"golang.org/x/exp/maps"
)

type NodeConfig struct {
	Flags map[string]interface{} `json:"flags"`
}

type NetworkConfig struct {
	NodeConfigs []NodeConfig           `json:"nodeConfigs"`
	CommonFlags map[string]interface{} `json:"commonFlags"`
	Upgrade     string                 `json:"upgrade"`
}

//go:embed default.json
var defaultNetworkData []byte

func GetDefaultNetworkConf(numNodes uint32) (
	*genesis.UnparsedConfig,
	[]byte,
	map[string]interface{},
	[]*tmpnet.Node,
	error,
) {
	networkConfig := NetworkConfig{}
	if err := json.Unmarshal(defaultNetworkData, &networkConfig); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failure unmarshaling network config from snapshot: %w", err)
	}
	nodes := []*tmpnet.Node{}
	for i := range numNodes {
		node := tmpnet.NewNode("")
		if int(i) < len(networkConfig.NodeConfigs) {
			nodeConfig := networkConfig.NodeConfigs[i]
			maps.Copy(node.Flags, nodeConfig.Flags)
		}
		if err := node.EnsureKeys(); err != nil {
			return nil, nil, nil, nil, err
		}
		nodes = append(nodes, node)
	}
	unparsedGenesis, err := tmpnet.NewTestGenesis(constants.LocalNetworkID, nodes, []*secp256k1.PrivateKey{
		genesis.EWOQKey,
	})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return unparsedGenesis, []byte(networkConfig.Upgrade), networkConfig.CommonFlags, nodes, nil
}
