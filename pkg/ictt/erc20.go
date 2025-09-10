// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ictt

import (
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm/contract"
	"github.com/ava-labs/libevm/common"
)

func GetTokenParams(
	rpcURL string,
	address common.Address,
) (string, string, uint8, error) {
	tokenName, err := GetTokenName(rpcURL, address)
	if err != nil {
		return "", "", 0, err
	}
	tokenSymbol, err := GetTokenSymbol(rpcURL, address)
	if err != nil {
		return "", "", 0, err
	}
	tokenDecimals, err := GetTokenDecimals(rpcURL, address)
	if err != nil {
		return "", "", 0, err
	}
	return tokenSymbol, tokenName, tokenDecimals, nil
}

func GetTokenDecimals(
	rpcURL string,
	address common.Address,
) (uint8, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"decimals()->(uint8)",
		nil,
	)
	if err != nil {
		return 0, err
	}
	return contract.GetSmartContractCallResult[uint8]("decimals", out)
}

func GetTokenName(
	rpcURL string,
	address common.Address,
) (string, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"name()->(string)",
		nil,
	)
	if err != nil {
		return "", err
	}
	return contract.GetSmartContractCallResult[string]("name", out)
}

func GetTokenSymbol(
	rpcURL string,
	address common.Address,
) (string, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		address,
		"symbol()->(string)",
		nil,
	)
	if err != nil {
		return "", err
	}
	return contract.GetSmartContractCallResult[string]("symbol", out)
}
