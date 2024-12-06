// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"testing"
)

func TestValidatorManager(t *testing.T) {
	rpc := "https://testnet-migration1-y28f1.avax-test.network/ext/bc/2jfUM5eofFf7gGoGLEhoo15USF5YgQwFeFcLUVT5aYSCZqiP5Y/rpc?token=9eb3b2556f29b22be75f608edb256a2b9d3cf873e7387e6d898f53d839579407"
	managerAddress := "0xd0C3b97B054f10b20e0c9AD9747d1F6968B21dEe"
	privateKey := ""
	subnetID := "QCj8NpcLz5u5291Qsz79GbxyJ5Aa8GtBQ43HQBVV5D6UaxbhQ"
	blockchainID := "2jfUM5eofFf7gGoGLEhoo15USF5YgQwFeFcLUVT5aYSCZqiP5Y"
	bootstrapValidator := []*txs.ConvertSubnetToL1Validator{}

	network := models.NewFujiNetwork()
	subnetConversionSignedMessage, err := GetPChainSubnetConversionWarpMessage(
		network,
		aggregatorLogLevel,
		0,
		true,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		bootstrapValidator,
	)

	tx, _, err := InitializeValidatorsSet(
		rpc,
		managerAddress,
		privateKey,
		subnetID,
		blockchainID,
		bootstrapValidator,
		subnetConversionSignedMessage,
	)
	if err != nil {
		fmt.Printf("err %s \n", err)
	}
}
