// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridgecmd

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
)

type PopularTokenInfo struct {
	TokenName            string
	TokenContractAddress string
	BridgeHubAddress     string
}

//go:embed popularTokensInfo.json
var popularTokensInfoByteSlice []byte

var popularTokensInfo map[string]map[string][]PopularTokenInfo

func (i PopularTokenInfo) Desc() string {
	switch {
	case i.TokenContractAddress != "" && i.BridgeHubAddress != "":
		return fmt.Sprintf("%s | Token address %s | Hub address %s", i.TokenName, i.TokenContractAddress, i.BridgeHubAddress)
	case i.BridgeHubAddress != "":
		return fmt.Sprintf("%s | Hub address %s", i.TokenName, i.BridgeHubAddress)
	default:
		return i.TokenName
	}
}

func GetPopularTokensInfo(network models.Network, blockchainAlias string) ([]PopularTokenInfo, error) {
	if err := json.Unmarshal(popularTokensInfoByteSlice, &popularTokensInfo); err != nil {
		return nil, fmt.Errorf("unabled to get popular tokens info from file: %w", err)
	}
	return popularTokensInfo[network.Kind.String()][blockchainAlias], nil
}
