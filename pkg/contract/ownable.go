// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	_ "embed"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// GetContractOwner gets owner for https://docs.openzeppelin.com/contracts/2.x/api/ownership#Ownable-owner contracts
func GetContractOwner(
	rpcURL string,
	contractAddress common.Address,
) (common.Address, error) {
	out, err := CallToMethod(
		rpcURL,
		contractAddress,
		"owner()->(address)",
	)
	if err != nil {
		return common.Address{}, err
	}

	ownerAddr, ok := out[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("error at owner() call, expected common.Address, got %T", out[0])
	}
	return ownerAddr, nil
}
