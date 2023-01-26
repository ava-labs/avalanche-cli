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
	"github.com/onsi/gomega"
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

	var chainConfig json.RawMessage
	if err := rpcClient.CallContext(ctx, &chainConfig, chainConfigAPI); err != nil {
		return err
	}

	var jsonToGo map[string]interface{}
	if err := json.Unmarshal(chainConfig, &jsonToGo); err != nil {
		return fmt.Errorf("failed to unpack JSON string to go map[string]interface{}")
	}

	upgradesI, ok := jsonToGo["upgrades"]
	if !ok {
		return errors.New("failed to find the 'upgrades' section in the JSON response")
	}

	serialized, err := json.Marshal(upgradesI)
	if err != nil {
		return fmt.Errorf("failed to serialize 'upgrades' section: %w", err)
	}

	var appliedUpgrades params.UpgradeConfig
	if err := json.Unmarshal(serialized, &appliedUpgrades); err != nil {
		return fmt.Errorf("failed to unpack JSON strings to params.UpgradeConfig")
	}

	gomega.Expect(appliedUpgrades).Should(gomega.Equal(deployedUpgrades))
	return nil
}
