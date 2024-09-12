// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"math/big"

	"github.com/ava-labs/subnet-evm/commontype"
)

const (
	DefaultEvmAirdropAmount = "1000000000000000000000000"
)

var (
	Difficulty = big.NewInt(0)

	// This is the current c-chain gas config
	StarterFeeConfig = commontype.FeeConfig{
		GasLimit:                 big.NewInt(8_000_000),
		MinBaseFee:               big.NewInt(25_000_000_000),
		TargetGas:                big.NewInt(15_000_000),
		BaseFeeChangeDenominator: big.NewInt(36),
		MinBlockGasCost:          big.NewInt(0),
		MaxBlockGasCost:          big.NewInt(1_000_000),
		TargetBlockRate:          2,
		BlockGasCostStep:         big.NewInt(200_000),
	}
)
