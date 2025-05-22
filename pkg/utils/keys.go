// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"math/big"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
)

func GetKeyNames(keyDir string, addEwoq bool) ([]string, error) {
	matches, err := os.ReadDir(keyDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, m := range matches {
		if strings.HasSuffix(m.Name(), constants.KeySuffix) {
			names = append(names, strings.TrimSuffix(m.Name(), constants.KeySuffix))
		}
	}
	userKeys := []string{}
	cliKeys := []string{}
	subnetKeys := []string{}
	for _, keyName := range names {
		switch {
		case strings.HasPrefix(keyName, "cli-"):
			cliKeys = append(cliKeys, keyName)
		case strings.HasPrefix(keyName, "subnet_"):
			subnetKeys = append(subnetKeys, keyName)
		default:
			userKeys = append(userKeys, keyName)
		}
	}
	if addEwoq {
		userKeys = append(userKeys, "ewoq")
	}
	names = append(append(userKeys, subnetKeys...), cliKeys...)
	return names, nil
}

func GetNetworkBalance(addressList []ids.ShortID, networkEndpoint string) (uint64, error) {
	ctx, cancel := GetAPIContext()
	defer cancel()
	pClient := platformvm.NewClient(networkEndpoint)
	bal, err := pClient.GetBalance(ctx, addressList)
	if err != nil {
		return 0, err
	}
	return uint64(bal.Balance), nil
}

func CalculateEvmFeeInAvax(gasUsed uint64, gasPrice *big.Int) float64 {
	gasUsedBig := new(big.Int).SetUint64(gasUsed)
	totalCost := new(big.Int).Mul(gasUsedBig, gasPrice)

	totalCostInNanoAvax := ConvertToNanoAvax(totalCost)

	result, _ := new(big.Float).SetInt(totalCostInNanoAvax).Float64()
	return result / float64(units.Avax)
}
