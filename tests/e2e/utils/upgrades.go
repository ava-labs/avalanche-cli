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

	// subnet-evm doesn't serialize the 'upgrades' section in a way that maps to go objects
	// (serialized with `json:"-"`)
	// therefore we need to do it manually:

	// first we get the response as a JSON string
	var chainConfig json.RawMessage
	if err := rpcClient.CallContext(ctx, &chainConfig, chainConfigAPI); err != nil {
		return err
	}

	// we want the "upgrades" section - easiest is to first unmarshal to a map...
	var jsonToGo map[string]interface{}
	if err := json.Unmarshal(chainConfig, &jsonToGo); err != nil {
		return fmt.Errorf("failed to unpack JSON string to go map[string]interface{}")
	}

	// ...then access the part we need...
	upgradesI, ok := jsonToGo["upgrades"]
	if !ok {
		return errors.New("failed to find the 'upgrades' section in the JSON response")
	}

	// then we serialize that back to a byte slice...
	serialized, err := json.Marshal(upgradesI)
	if err != nil {
		return fmt.Errorf("failed to serialize 'upgrades' section: %w", err)
	}

	// ...so that we finally can unmarshal to the object we need
	var appliedUpgrades params.UpgradeConfig
	if err := json.Unmarshal(serialized, &appliedUpgrades); err != nil {
		return fmt.Errorf("failed to unpack JSON strings to params.UpgradeConfig")
	}

	gomega.Expect(appliedUpgrades).Should(gomega.Equal(deployedUpgrades))
	return nil
}
