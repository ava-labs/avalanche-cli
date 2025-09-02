// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm/contract"
	validatorManagerSDK "github.com/ava-labs/avalanche-tooling-sdk-go/validatormanager"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
)

//go:embed smart_contracts/transparent_proxy_bytecode.txt
var transparentProxyBytecode []byte

func DeployTransparentProxy(
	rpcURL string,
	privateKey string,
	implementation common.Address,
	admin common.Address,
) (common.Address, common.Address, *types.Transaction, *types.Receipt, error) {
	transparentProxyBytes := []byte(strings.TrimSpace(string(transparentProxyBytecode)))
	proxy, tx, receipt, err := contract.DeployContract(
		rpcURL,
		privateKey,
		transparentProxyBytes,
		"(address, address, bytes)",
		implementation,
		admin,
		[]byte{},
	)
	if err != nil {
		return proxy, common.Address{}, tx, receipt, err
	}
	event, err := evm.GetEventFromLogs(receipt.Logs, ParseAdminChanged)
	if err != nil {
		return proxy, common.Address{}, tx, receipt, fmt.Errorf("failed to get proxy admin event: %w", err)
	}
	proxyAdmin := event.NewAdmin
	return proxy, proxyAdmin, tx, receipt, err
}

func SetupProxyImplementation(
	logger logging.Logger,
	rpcURL string,
	proxyAdminContractAddress common.Address,
	transparentProxyContractAddress common.Address,
	proxyOwnerPrivateKey string,
	implementation common.Address,
	description string,
) (*types.Transaction, *types.Receipt, error) {
	useUpgradeAndCall := false
	if out, err := contract.CallToMethod(
		rpcURL,
		proxyAdminContractAddress,
		"UPGRADE_INTERFACE_VERSION()->(string)",
		nil,
	); err == nil {
		if v, err := contract.GetSmartContractCallResult[string]("UPGRADE_INTERFACE_VERSION", out); err == nil && v == "5.0.0" {
			useUpgradeAndCall = true
		}
	}
	if useUpgradeAndCall {
		return contract.TxToMethod(
			logger,
			rpcURL,
			false,
			common.Address{},
			proxyOwnerPrivateKey,
			proxyAdminContractAddress,
			big.NewInt(0),
			description,
			validatorManagerSDK.ErrorSignatureToError,
			"upgradeAndCall(address,address,bytes)",
			transparentProxyContractAddress,
			implementation,
			[]byte{},
		)
	}
	return contract.TxToMethod(
		logger,
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
		nil,
		transparentProxyContractAddress,
	)
	if err != nil {
		return common.Address{}, err
	}
	return contract.GetSmartContractCallResult[common.Address]("getProxyImplementation", out)
}

type TransparentUpgradeableProxyAdminChanged struct {
	PreviousAdmin common.Address
	NewAdmin      common.Address
	Raw           types.Log
}

func ParseAdminChanged(log types.Log) (*TransparentUpgradeableProxyAdminChanged, error) {
	event := new(TransparentUpgradeableProxyAdminChanged)
	if err := contract.UnpackLog(
		"AdminChanged(address,address)",
		[]int{},
		log,
		event,
	); err != nil {
		return nil, err
	}
	return event, nil
}

func SetupGenesisValidatorProxyImplementation(
	logger logging.Logger,
	rpcURL string,
	proxyOwnerPrivateKey string,
	validatorManager common.Address,
) (*types.Transaction, *types.Receipt, error) {
	return SetupProxyImplementation(
		logger,
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
	logger logging.Logger,
	rpcURL string,
	proxyOwnerPrivateKey string,
	validatorManager common.Address,
) (*types.Transaction, *types.Receipt, error) {
	return SetupProxyImplementation(
		logger,
		rpcURL,
		common.HexToAddress(validatorManagerSDK.SpecializationProxyAdminContractAddress),
		common.HexToAddress(validatorManagerSDK.SpecializationProxyContractAddress),
		proxyOwnerPrivateKey,
		validatorManager,
		"set specialization proxy implementation",
	)
}
