// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransferAmountInBaseUnits(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		decimals uint8
		want     string
	}{
		{"six decimals", 5, 6, "5000000"},
		{"fractional token", 0.05, 6, "50000"},
		{"one base unit", 0.000001, 6, "1"},
		{"eighteen decimals", 0.05, 18, "50000000000000000"},
		{"zero decimals", 5, 0, "5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transferAmountInBaseUnits(tt.amount, tt.decimals)
			require.NoError(t, err)
			require.Equal(t, tt.want, got.String())
		})
	}
}

func TestTransferAmountInBaseUnitsRejectsInvalidAmounts(t *testing.T) {
	for _, amount := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		_, err := transferAmountInBaseUnits(amount, 6)
		require.ErrorContains(t, err, "finite and greater than zero")
	}
	_, err := transferAmountInBaseUnits(0.0000001, 6)
	require.ErrorContains(t, err, "cannot be represented with 6 decimals")
	_, err = transferAmountInBaseUnits(1e80, 18)
	require.ErrorContains(t, err, "uint256 limit")
}
