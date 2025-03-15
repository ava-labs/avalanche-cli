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

type nodeConfig struct {
	Flags map[string]interface{} `json:"flags"`
}

type networkConfig struct {
	NodeConfigs []nodeConfig           `json:"nodeConfigs"`
	CommonFlags map[string]interface{} `json:"commonFlags"`
	Upgrade     string                 `json:"upgrade"`
}

//go:embed default.json
var defaultNetworkData []byte

// GetDefaultNetworkConf creates a default network configuration of [numNodes]
// compatible with TmpNet usage, where the first len(networkConf.NodeConfigs) /== 5/
// will have default local network NodeID/BLSInfo/Ports, and the remaining
// ones will be dynamically generated.
// It returns the local network's:
// - genesis
// - upgrade
// - common flags
// - node confs
func GetDefaultNetworkConf(numNodes uint32) (
	uint32,
	*genesis.UnparsedConfig,
	[]byte,
	map[string]interface{},
	[]*tmpnet.Node,
	error,
) {
	networkConf := networkConfig{}
	if err := json.Unmarshal(defaultNetworkData, &networkConf); err != nil {
		return 0, nil, nil, nil, nil, fmt.Errorf("failure unmarshaling default local network config: %w", err)
	}
	nodes := []*tmpnet.Node{}
	for i := range numNodes {
		node := tmpnet.NewNode("")
		if int(i) < len(networkConf.NodeConfigs) {
			maps.Copy(node.Flags, networkConf.NodeConfigs[i].Flags)
		}
		if err := node.EnsureKeys(); err != nil {
			return 0, nil, nil, nil, nil, err
		}
		nodes = append(nodes, node)
	}
	unparsedGenesis, err := tmpnet.NewTestGenesis(constants.LocalNetworkID, nodes, []*secp256k1.PrivateKey{
		genesis.EWOQKey,
	})
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}
	return constants.LocalNetworkID, unparsedGenesis, []byte(networkConf.Upgrade), networkConf.CommonFlags, nodes, nil
}
