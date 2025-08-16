// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package pchain

import (
	"github.com/ava-labs/avalanche-cli/sdk/wallet"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	avagofee "github.com/ava-labs/avalanchego/vms/platformvm/txs/fee"
)

// GetTxFee returns the fee to be paid by a given Tx
// client(wallet) should contain updated gas price information
func GetTxFee(
	client wallet.Wallet,
	unsignedTx txs.UnsignedTx,
) (uint64, error) {
	pContext := client.P().Builder().Context()
	pFeeCalculator := avagofee.NewDynamicCalculator(pContext.ComplexityWeights, pContext.GasPrice)
	return pFeeCalculator.CalculateFee(unsignedTx)
}
