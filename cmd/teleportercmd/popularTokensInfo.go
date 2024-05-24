// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

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
	if i.TokenContractAddress == "" {
		return i.TokenName
	} else {
		return fmt.Sprintf("%s | Token address %s | Hub bridge address %s", i.TokenName, i.TokenContractAddress, i.BridgeHubAddress)
	}
}

func GetPopularTokensInfo(network models.Network, blockchainAlias string) ([]PopularTokenInfo, error) {
	if err := json.Unmarshal(popularTokensInfoByteSlice, &popularTokensInfo); err != nil {
		return nil, fmt.Errorf("unabled to get popular tokens info from file: %w", err)
	}
	if network.Kind == models.Fuji {
		return popularTokensInfo[models.Fuji.String()][blockchainAlias], nil
	} else {
		return nil, nil
	}
}
