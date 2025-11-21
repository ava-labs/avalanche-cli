// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm/contract"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/core/types"
)

//go:embed contracts/bin/Token.bin
var tokenBin []byte

//go:embed contracts/bin/MintableERC20.bin
var mintableERC20Bin []byte

func DeployERC20(
	rpcURL string,
	signer *evm.Signer,
	symbol string,
	funded common.Address,
	supply *big.Int,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	return contract.DeployContract(
		rpcURL,
		signer,
		tokenBin,
		"(string, address, uint256)",
		symbol,
		funded,
		supply,
	)
}

func DeployMintableERC20(
	rpcURL string,
	signer *evm.Signer,
	symbol string,
	funded common.Address,
	supply *big.Int,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	return contract.DeployContract(
		rpcURL,
		signer,
		mintableERC20Bin,
		"(string, address, uint256)",
		symbol,
		funded,
		supply,
	)
}
