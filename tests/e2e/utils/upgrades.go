// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"context"
	"encoding/json"
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

func CheckUpgradeIsDeployed(rpcEndpoint string, deployedUpgrade params.PrecompileUpgrade) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
	defer cancel()

	rpcClient, err := rpc.DialContext(ctx, rpcEndpoint)
	if err != nil {
		return err
	}

	var result json.RawMessage
	if err := rpcClient.CallContext(ctx, &result, chainConfigAPI); err != nil {
		return err
	}

	var chainConfig params.ChainConfig
	if err := json.Unmarshal(result, &chainConfig); err != nil {
		return fmt.Errorf("failed to unpack API response into a subnet-evm/params.ChainConfig: %w", err)
	}

	upgrades := chainConfig.UpgradeConfig.PrecompileUpgrades
	found := false
	for _, upgrade := range upgrades {
		if cmp.Equal(deployedUpgrade, upgrade) {
			found = true
		}
	}
	if !found {
		return errors.New("API did not report the upgrade in its config")
	}
	return nil
}
