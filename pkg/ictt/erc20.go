// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ictt

import (
	_ "embed"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/liyue201/erc20-go/erc20"
)

func GetTokenParams(endpoint string, tokenAddress common.Address) (string, string, uint8, error) {
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return "", "", 0, err
	}
	token, err := erc20.NewGGToken(tokenAddress, client)
	if err != nil {
		return "", "", 0, err
	}
	tokenName, err := token.Name(nil)
	if err != nil {
		return "", "", 0, err
	}
	tokenSymbol, err := token.Symbol(nil)
	if err != nil {
		return "", "", 0, err
	}
	tokenDecimals, err := token.Decimals(nil)
	if err != nil {
		return "", "", 0, err
	}
	return tokenSymbol, tokenName, tokenDecimals, nil
}

func GetTokenDecimals(endpoint string, tokenAddress common.Address) (uint8, error) {
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return 0, err
	}
	token, err := erc20.NewGGToken(tokenAddress, client)
	if err != nil {
		return 0, err
	}
	return token.Decimals(nil)
}
