// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
)

//go:embed smart_contracts/proxy_admin_bytecode.txt
var proxyAdminBytecode []byte

func DeployProxyAdmin(
	rpcURL string,
	privateKey string,
	owner common.Address,
) (common.Address, error) {
	proxyAdminBytes := []byte(strings.TrimSpace(string(proxyAdminBytecode)))
	return contract.DeployContract(
		rpcURL,
		privateKey,
		proxyAdminBytes,
		"(address)",
		owner,
	)
}

//go:embed smart_contracts/transparent_proxy_bytecode.txt
var transparentProxyBytecode []byte

func DeployTransparentProxy(
	rpcURL string,
	privateKey string,
	implementation common.Address,
	admin common.Address,
) (common.Address, error) {
	transparentProxyBytes := []byte(strings.TrimSpace(string(transparentProxyBytecode)))
	return contract.DeployContract(
		rpcURL,
		privateKey,
		transparentProxyBytes,
		"(address, address, bytes)",
		implementation,
		admin,
		[]byte{},
	)
}

func SetupProxyImplementation(
	rpcURL string,
	proxyAdminContractAddress common.Address,
	transparentProxyContractAddress common.Address,
	proxyOwnerPrivateKey string,
	implementation common.Address,
	description string,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethod(
		rpcURL,
		false,
		common.Address{},
		proxyOwnerPrivateKey,
		proxyAdminContractAddress,
		big.NewInt(0),
		description,
		validatorManagerSDK.ErrorSignatureToError,
		"upgrade(address,address)",
		transparentProxyContractAddress,
		implementation,
	)
}

func GetProxyImplementation(
	rpcURL string,
	proxyAdminContractAddress common.Address,
	transparentProxyContractAddress common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		proxyAdminContractAddress,
		"getProxyImplementation(address)->(address)",
		transparentProxyContractAddress,
	)
	if err != nil {
		return common.Address{}, err
	}
	return contract.GetSmartContractCallResult[common.Address]("getProxyImplementation", out)
}


func SetupGenesisValidatorProxyImplementation(
	rpcURL string,
	proxyOwnerPrivateKey string,
	validatorManager common.Address,
) (*types.Transaction, *types.Receipt, error) {
	return SetupProxyImplementation(
		rpcURL,
		common.HexToAddress(validatorManagerSDK.ValidatorProxyAdminContractAddress),
		common.HexToAddress(validatorManagerSDK.ValidatorProxyContractAddress),
		proxyOwnerPrivateKey,
		validatorManager,
		"set validator proxy implementation",
	)
}

func GetGenesisValidatorProxyImplementation(
	rpcURL string,
) (common.Address, error) {
	return GetProxyImplementation(
		rpcURL,
		common.HexToAddress(validatorManagerSDK.ValidatorProxyAdminContractAddress),
		common.HexToAddress(validatorManagerSDK.ValidatorProxyContractAddress),
	)
}

func GenesisValidatorProxyHasImplementationSet(
	rpcURL string,
) (bool, error) {
	validatorManagerAddress, err := GetGenesisValidatorProxyImplementation(rpcURL)
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

func GetGenesisSpecializedValidatorProxyImplementation(
	rpcURL string,
) (common.Address, error) {
	return GetProxyImplementation(
		rpcURL,
		common.HexToAddress(validatorManagerSDK.SpecializationProxyAdminContractAddress),
		common.HexToAddress(validatorManagerSDK.SpecializationProxyContractAddress),
	)
}

func SetupGenesisSpecializationProxyImplementation(
	rpcURL string,
	proxyOwnerPrivateKey string,
	validatorManager common.Address,
) (*types.Transaction, *types.Receipt, error) {
	return SetupProxyImplementation(
		rpcURL,
		common.HexToAddress(validatorManagerSDK.SpecializationProxyAdminContractAddress),
		common.HexToAddress(validatorManagerSDK.SpecializationProxyContractAddress),
		proxyOwnerPrivateKey,
		validatorManager,
		"set specialization proxy implementation",
	)
}
