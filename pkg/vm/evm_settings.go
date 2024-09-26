// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var (
	// current avacloud settings
	LowGasLimit     = big.NewInt(12_000_000)
	MediumGasLimit  = big.NewInt(15_000_000) // C-Chain value
	HighGasLimit    = big.NewInt(20_000_000)
	LowTargetGas    = big.NewInt(25_000_000) // ~ 2.1x of gas limit
	MediumTargetGas = big.NewInt(45_000_000) // 3x of gas limit (also, 3x bigger than C-Chain)
	HighTargetGas   = big.NewInt(60_000_000) // 3x of gas limit

	NoDynamicFeesGasLimitToTargetGasFactor = big.NewInt(5)

	PrefundedEwoqAddress = common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")
	PrefundedEwoqPrivate = "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"

	OneAvax                 = new(big.Int).SetUint64(1000000000000000000)
	defaultEVMAirdropAmount = new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil) // 10^24
	defaultPoAOwnerBalance  = new(big.Int).Mul(OneAvax, big.NewInt(10))             // 10 Native Tokens
)
