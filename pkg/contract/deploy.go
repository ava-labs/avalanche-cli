// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/avalanche-tooling-sdk-go/evm/contract"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/core/types"
)

//go:embed contracts/bin/Token.bin
var tokenBin []byte

func DeployERC20(
	rpcURL string,
	privateKey string,
	symbol string,
	funded common.Address,
	supply *big.Int,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	return contract.DeployContract(
		rpcURL,
		privateKey,
		tokenBin,
		"(string, address, uint256)",
		symbol,
		funded,
		supply,
	)
}
