// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tokentransferrercmd

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
)

type PopularTokenInfo struct {
	TokenName              string
	TokenContractAddress   string
	TransferrerHomeAddress string
}

//go:embed popularTokensInfo.json
var popularTokensInfoByteSlice []byte

var popularTokensInfo map[string]map[string][]PopularTokenInfo

func (i PopularTokenInfo) Desc() string {
	switch {
	case i.TokenContractAddress != "" && i.TransferrerHomeAddress != "":
		return fmt.Sprintf("%s | Token address %s | Home address %s", i.TokenName, i.TokenContractAddress, i.TransferrerHomeAddress)
	case i.TransferrerHomeAddress != "":
		return fmt.Sprintf("%s | Home address %s", i.TokenName, i.TransferrerHomeAddress)
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
