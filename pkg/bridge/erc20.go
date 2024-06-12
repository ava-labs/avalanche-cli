// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridge

import (
	_ "embed"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/liyue201/erc20-go/erc20"
)

func GetTokenParams(endpoint string, tokenAddress string) (string, string, uint8, error) {
	address := common.HexToAddress(tokenAddress)
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return "", "", 0, err
	}
	token, err := erc20.NewGGToken(address, client)
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
	// TODO: find out if there are decimals options and why (academy)
	tokenDecimals, err := token.Decimals(nil)
	if err != nil {
		return "", "", 0, err
	}
	return tokenSymbol, tokenName, tokenDecimals, nil
}
