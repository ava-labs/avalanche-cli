// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/avalanche-cli/sdk/contract"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
)

func SetupValidatorProxyImplementation(
	logger logging.Logger,
	rpcURL string,
	proxyManagerPrivateKey string,
	validatorManager common.Address,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethod(
		logger,
		rpcURL,
		false,
		common.Address{},
		proxyManagerPrivateKey,
		common.HexToAddress(validatorManagerSDK.ValidatorProxyAdminContractAddress),
		big.NewInt(0),
		"set validator proxy implementation",
		validatorManagerSDK.ErrorSignatureToError,
		"upgrade(address,address)",
		common.HexToAddress(validatorManagerSDK.ValidatorProxyContractAddress),
		validatorManager,
	)
}

func GetValidatorProxyImplementation(
	rpcURL string,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		common.HexToAddress(validatorManagerSDK.ValidatorProxyAdminContractAddress),
		"getProxyImplementation(address)->(address)",
		common.HexToAddress(validatorManagerSDK.ValidatorProxyContractAddress),
	)
	if err != nil {
		return common.Address{}, err
	}
	return contract.GetSmartContractCallResult[common.Address]("getProxyImplementation", out)
}

func ValidatorProxyHasImplementationSet(
	rpcURL string,
) (bool, error) {
	validatorManagerAddress, err := GetValidatorProxyImplementation(rpcURL)
	if err != nil {
		return false, err
	}
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return false, err
	}
	return client.ContractAlreadyDeployed(
		validatorManagerAddress.Hex(),
	)
}

func GetSpecializedValidatorProxyImplementation(
	rpcURL string,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		common.HexToAddress(validatorManagerSDK.SpecializationProxyAdminContractAddress),
		"getProxyImplementation(address)->(address)",
		common.HexToAddress(validatorManagerSDK.SpecializationProxyContractAddress),
	)
	if err != nil {
		return common.Address{}, err
	}
	return contract.GetSmartContractCallResult[common.Address]("getProxyImplementation", out)
}

func SetupSpecializationProxyImplementation(
	logger logging.Logger,
	rpcURL string,
	proxyManagerPrivateKey string,
	validatorManager common.Address,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethod(
		logger,
		rpcURL,
		false,
		common.Address{},
		proxyManagerPrivateKey,
		common.HexToAddress(validatorManagerSDK.SpecializationProxyAdminContractAddress),
		big.NewInt(0),
		"set specialization proxy implementation",
		validatorManagerSDK.ErrorSignatureToError,
		"upgrade(address,address)",
		common.HexToAddress(validatorManagerSDK.SpecializationProxyContractAddress),
		validatorManager,
	)
}
