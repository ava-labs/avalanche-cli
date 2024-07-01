// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	_ "embed"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

//go:embed contracts/bin/Token.bin
var tokenBin []byte

func DeployERC20(
	rpcURL string,
	privateKey string,
	symbol string,
	funded common.Address,
	supply *big.Int,
) (common.Address, error) {
	return DeployContract(
		rpcURL,
		privateKey,
		tokenBin,
		"(string, address, uint256)",
		symbol,
		funded,
		supply,
	)
}
