// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/rpc"
	"github.com/google/go-cmp/cmp"
)

const (
	chainConfigAPI = "eth_getChainConfig"
)

func CheckUpgradeIsDeployed(rpcEndpoint string, deployedUpgrades params.UpgradeConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
	defer cancel()

	rpcClient, err := rpc.DialContext(ctx, rpcEndpoint)
	if err != nil {
		return err
	}

	var chainConfig params.ChainConfig
	if err := rpcClient.CallContext(ctx, &chainConfig, chainConfigAPI); err != nil {
		return err
	}
	fmt.Println(chainConfig)

	upgrades := chainConfig.UpgradeConfig
	fmt.Println(deployedUpgrades)
	fmt.Println(upgrades)
	// found := false
	//	for _, upgrade := range upgrades {
	if !cmp.Equal(deployedUpgrades, upgrades) {
		//			found = true
		//}
		//	}
		//if !found {
		return errors.New("API did not report the upgrade in its config")
	}
	return nil
}
