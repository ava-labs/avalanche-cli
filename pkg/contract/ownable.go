// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import "github.com/ethereum/go-ethereum/common"

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
	return GetSmartContractCallResult[common.Address]("owner", out)
}

func TransferOwnership(
	rpcURL string,
	contractAddress common.Address,
	ownerPrivateKey string,
	newOwner common.Address,
) error {
	_, _, err := TxToMethod(
		rpcURL,
		false,
		common.Address{},
		ownerPrivateKey,
		contractAddress,
		nil,
		"transfer ownership",
		nil,
		"transferOwnership(address)",
		newOwner,
	)
	return err
}
