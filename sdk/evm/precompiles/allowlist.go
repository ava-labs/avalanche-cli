// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package precompiles

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ethereum/go-ethereum/common"
)

func SetAdmin(
	rpcURL string,
	precompile common.Address,
	privateKey string,
	toSet common.Address,
) error {
	_, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		precompile,
		nil,
		"set precompile admin",
		nil,
		"setAdmin(address)",
		toSet,
	)
	return err
}

func SetManager(
	rpcURL string,
	precompile common.Address,
	privateKey string,
	toSet common.Address,
) error {
	_, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		precompile,
		nil,
		"set precompile manager",
		nil,
		"setManager(address)",
		toSet,
	)
	return err
}

func SetEnabled(
	rpcURL string,
	precompile common.Address,
	privateKey string,
	toSet common.Address,
) error {
	_, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		precompile,
		nil,
		"set precompile enabled",
		nil,
		"setEnabled(address)",
		toSet,
	)
	return err
}

func SetNone(
	rpcURL string,
	precompile common.Address,
	privateKey string,
	toSet common.Address,
) error {
	_, _, err := contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		privateKey,
		precompile,
		nil,
		"set precompile none",
		nil,
		"setNone(address)",
		toSet,
	)
	return err
}

func ReadAllowList(
	rpcURL string,
	precompile common.Address,
	toQuery common.Address,
) (*big.Int, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		precompile,
		"readAllowList(address)->(uint256)",
		toQuery,
	)
	if err != nil {
		return nil, err
	}
	return contract.GetSmartContractCallResult[*big.Int]("readAllowList", out)
}
