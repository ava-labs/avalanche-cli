// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
)

func SetupValidatorManagerAtProxy(
	rpcURL string,
	proxyManagerPrivateKey string,
	validatorManager common.Address,
) (*types.Transaction, *types.Receipt, error) {
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return nil, nil, err
	}
	fmt.Println(evm.ContractAlreadyDeployed(client, validatorManager.Hex()))
	return contract.TxToMethod(
		rpcURL,
		proxyManagerPrivateKey,
		common.HexToAddress(validatorManagerSDK.ProxyAdminContractAddress),
		big.NewInt(0),
		"set proxy to PoS",
		validatorManagerSDK.ErrorSignatureToError,
		"upgrade(address,address)",
		common.HexToAddress(validatorManagerSDK.ProxyContractAddress),
		validatorManager,
	)
}

func GetProxyValidatorManager(
	rpcURL string,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		common.HexToAddress(validatorManagerSDK.ProxyAdminContractAddress),
		"getProxyImplementation(address)->(address)",
		common.HexToAddress(validatorManagerSDK.ProxyContractAddress),
	)
	if err != nil {
		return common.Address{}, err
	}
	validatorManagerAddress, b := out[0].(common.Address)
	if !b {
		return common.Address{}, fmt.Errorf("error obtaining proxy implementation, expected common.Address, got %T", out[0])
	}
	return validatorManagerAddress, nil
}

func ProxyHasValidatorManagerSet(
	rpcURL string,
) (bool, error) {
	validatorManagerAddress, err := GetProxyValidatorManager(rpcURL)
	if err != nil {
		return false, err
	}
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return false, err
	}
	return evm.ContractAlreadyDeployed(
		client,
		validatorManagerAddress.Hex(),
	)
}
