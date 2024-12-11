// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"math/big"
)

const defaultDenomination = 18

// Convert an integed amount of the given denomination to base units
// (i.e. An amount of 54 with a decimals value of 3 results in 54000)
func ApplyDenomination(amount uint64, decimals uint8) *big.Int {
	multiplier := new(big.Int).Exp(
		big.NewInt(10),
		big.NewInt(int64(decimals)),
		nil,
	)
	return new(big.Int).Mul(
		big.NewInt(int64(amount)),
		multiplier,
	)
}

// Convert an integed amount of the default denomination to base units
func ApplyDefaultDenomination(amount uint64) *big.Int {
	return ApplyDenomination(amount, defaultDenomination)
}
