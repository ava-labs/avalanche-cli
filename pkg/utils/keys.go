// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"
	"github.com/ava-labs/avalanchego/ids"
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
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	pClient := platformvm.NewClient(networkEndpoint)
	bal, err := pClient.GetBalance(ctx, addressList)
	if err != nil {
		return 0, err
	}
	return uint64(bal.Balance), nil
}
