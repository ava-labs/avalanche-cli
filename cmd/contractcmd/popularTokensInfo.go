// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

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

var popularTokensInfo map[string][]PopularTokenInfo

func (i PopularTokenInfo) Desc() string {
	if i.TokenContractAddress == "" {
		return i.TokenName
	} else {
		return fmt.Sprintf("%s | Token address %s | Hub bridge address %s", i.TokenName, i.TokenContractAddress, i.BridgeHubAddress)
	}
}

func GetPopularTokensInfo(network models.Network, subnetOption string) ([]PopularTokenInfo, error) {
	if err := json.Unmarshal(popularTokensInfoByteSlice, &popularTokensInfo); err != nil {
		return nil, fmt.Errorf("unabled to get popular tokens info from file: %w", err)
	}
	if network.Kind == models.Fuji && subnetOption == "C-Chain" {
		return popularTokensInfo[models.Fuji.String()], nil
	} else {
		return nil, nil
	}
}
