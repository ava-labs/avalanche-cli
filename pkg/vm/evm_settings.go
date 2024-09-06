// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"math/big"

	"github.com/ava-labs/subnet-evm/commontype"
	"github.com/ethereum/go-ethereum/common"
)

var (
	Difficulty = big.NewInt(0)

	// current avacloud settings
	LowGasLimit     = big.NewInt(12_000_000)
	MediumGasLimit  = big.NewInt(15_000_000) // C-Chain value
	HighGasLimit    = big.NewInt(20_000_000)
	LowTargetGas    = big.NewInt(25_000_000) // ~ 2.1x of gas limit
	MediumTargetGas = big.NewInt(45_000_000) // 3x of gas limit (also, 3x bigger than C-Chain)
	HighTargetGas   = big.NewInt(60_000_000) // 3x of gas limit

	NoDynamicFeesGasLimitToTargetGasFactor = big.NewInt(5)

	// This is the current c-chain gas config
	StarterFeeConfig = commontype.FeeConfig{
		GasLimit:                 big.NewInt(15_000_000),
		MinBaseFee:               big.NewInt(25_000_000_000),
		TargetGas:                big.NewInt(15_000_000),
		BaseFeeChangeDenominator: big.NewInt(36),
		MinBlockGasCost:          big.NewInt(0),
		MaxBlockGasCost:          big.NewInt(1_000_000),
		TargetBlockRate:          2,
		BlockGasCostStep:         big.NewInt(200_000),
	}

	PrefundedEwoqAddress = common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")
	PrefundedEwoqPrivate = "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"

	oneAvax                 = new(big.Int).SetUint64(1000000000000000000)
	defaultEVMAirdropAmount = new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil) // 10^24
)
