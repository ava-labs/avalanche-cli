// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	validatormanagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
)

//go:embed smart_contracts/deployed_validator_messages_bytecode_v2.0.0.txt
var deployedValidatorMessagesV2_0_0Bytecode []byte

func AddValidatorMessagesV2_0_0ContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedValidatorMessagesBytes := common.FromHex(strings.TrimSpace(string(deployedValidatorMessagesV2_0_0Bytecode)))
	allocs[common.HexToAddress(validatormanagerSDK.ValidatorMessagesContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedValidatorMessagesBytes,
		Nonce:   1,
	}
}

//go:embed smart_contracts/validator_messages_bytecode_v2.0.0.txt
var validatorMessagesV2_0_0Bytecode []byte

func DeployValidatorMessagesV2_0_0Contract(
	rpcURL string,
	privateKey string,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	validatorMessagesBytes := []byte(strings.TrimSpace(string(validatorMessagesV2_0_0Bytecode)))
	return contract.DeployContract(
		rpcURL,
		privateKey,
		validatorMessagesBytes,
		"()",
	)
}

func hasValidatorMessagesAddressPlaceholder(
	contract string,
) bool {
	return strings.Contains(
		contract,
		"__$fd0c147b4031eef6079b0498cbafa865f0$__",
	)
}

func fillValidatorMessagesAddressPlaceholder(
	contract string,
	messagesContractAddress string,
) string {
	return strings.ReplaceAll(
		contract,
		"__$fd0c147b4031eef6079b0498cbafa865f0$__",
		messagesContractAddress[2:],
	)
}

func fillGenesisValidatorMessagesAddressPlaceholder(
	contract string,
) string {
	return fillValidatorMessagesAddressPlaceholder(contract, validatormanagerSDK.ValidatorMessagesContractAddress)
}

//go:embed smart_contracts/deployed_poa_validator_manager_bytecode_v1.0.0.txt
var deployedPoAValidatorManagerV1_0_0Bytecode []byte

func AddPoAValidatorManagerV1_0_0ContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedPoaValidatorManagerString := strings.TrimSpace(string(deployedPoAValidatorManagerV1_0_0Bytecode))
	deployedPoaValidatorManagerString = fillGenesisValidatorMessagesAddressPlaceholder(deployedPoaValidatorManagerString)
	deployedPoaValidatorManagerBytes := common.FromHex(deployedPoaValidatorManagerString)
	allocs[common.HexToAddress(validatormanagerSDK.ValidatorContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedPoaValidatorManagerBytes,
		Nonce:   1,
	}
}

//go:embed smart_contracts/deployed_validator_manager_bytecode_v2.0.0.txt
var deployedValidatorManagerV2_0_0Bytecode []byte

func AddValidatorManagerV2_0_0ContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedValidatorManagerString := strings.TrimSpace(string(deployedValidatorManagerV2_0_0Bytecode))
	deployedValidatorManagerString = fillGenesisValidatorMessagesAddressPlaceholder(deployedValidatorManagerString)
	deployedValidatorManagerBytes := common.FromHex(deployedValidatorManagerString)
	allocs[common.HexToAddress(validatormanagerSDK.ValidatorContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedValidatorManagerBytes,
		Nonce:   1,
	}
}

func DeployValidatorManagerContract(
	rpcURL string,
	privateKey string,
	validatorMessagesAtGenesis bool,
	validatorManagerBytecode string,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	validatorManagerString := strings.TrimSpace(validatorManagerBytecode)
	if hasValidatorMessagesAddressPlaceholder(validatorManagerString) {
		if validatorMessagesAtGenesis {
			validatorManagerString = fillGenesisValidatorMessagesAddressPlaceholder(validatorManagerString)
		} else {
			validatorMessagesContractAddress, _, _, err := DeployValidatorMessagesV2_0_0Contract(rpcURL, privateKey)
			if err != nil {
				return common.Address{}, nil, nil, err
			}
			validatorManagerString = fillValidatorMessagesAddressPlaceholder(validatorManagerString, validatorMessagesContractAddress.Hex())
		}
	}
	validatorManagerBytes := []byte(validatorManagerString)
	return contract.DeployContract(
		rpcURL,
		privateKey,
		validatorManagerBytes,
		"(uint8)",
		uint8(0),
	)
}

//go:embed smart_contracts/validator_manager_bytecode_v2.0.0.txt
var validatorManagerV2_0_0Bytecode []byte

func DeployValidatorManagerV2_0_0Contract(
	rpcURL string,
	privateKey string,
	validatorMessagesAtGenesis bool,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	return DeployValidatorManagerContract(
		rpcURL,
		privateKey,
		validatorMessagesAtGenesis,
		string(validatorManagerV2_0_0Bytecode),
	)
}

func DeployValidatorManagerV2_0_0ContractAndRegisterAtGenesisProxy(
	rpcURL string,
	privateKey string,
	validatorMessagesAtGenesis bool,
	proxyOwnerPrivateKey string,
) (common.Address, error) {
	validatorManagerAddress, _, _, err := DeployValidatorManagerV2_0_0Contract(
		rpcURL,
		privateKey,
		validatorMessagesAtGenesis,
	)
	if err != nil {
		return common.Address{}, err
	}
	if _, _, err := SetupGenesisValidatorProxyImplementation(
		rpcURL,
		proxyOwnerPrivateKey,
		validatorManagerAddress,
	); err != nil {
		return common.Address{}, err
	}
	return validatorManagerAddress, nil
}

//go:embed smart_contracts/native_token_staking_manager_bytecode_v1.0.0.txt
var posValidatorManagerV1_0_0Bytecode []byte

func DeployPoSValidatorManagerV1_0_0Contract(
	rpcURL string,
	privateKey string,
	validatorMessagesAtGenesis bool,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	return DeployValidatorManagerContract(
		rpcURL,
		privateKey,
		validatorMessagesAtGenesis,
		string(posValidatorManagerV1_0_0Bytecode),
	)
}

func DeployPoSValidatorManagerV1_0_0ContractAndRegisterAtGenesisProxy(
	rpcURL string,
	privateKey string,
	validatorMessagesAtGenesis bool,
	proxyOwnerPrivateKey string,
) (common.Address, error) {
	posValidatorManagerAddress, _, _, err := DeployPoSValidatorManagerV1_0_0Contract(
		rpcURL,
		privateKey,
		validatorMessagesAtGenesis,
	)
	if err != nil {
		return common.Address{}, err
	}
	if _, _, err := SetupGenesisValidatorProxyImplementation(
		rpcURL,
		proxyOwnerPrivateKey,
		posValidatorManagerAddress,
	); err != nil {
		return common.Address{}, err
	}
	return posValidatorManagerAddress, nil
}

//go:embed smart_contracts/native_token_staking_manager_bytecode_v2.0.0.txt
var posValidatorManagerV2_0_0Bytecode []byte

func DeployPoSValidatorManagerV2_0_0Contract(
	rpcURL string,
	privateKey string,
	validatorMessagesAtGenesis bool,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	return DeployValidatorManagerContract(
		rpcURL,
		privateKey,
		validatorMessagesAtGenesis,
		string(posValidatorManagerV2_0_0Bytecode),
	)
}

func DeployPoSValidatorManagerV2_0_0ContractAndRegisterAtGenesisProxy(
	rpcURL string,
	privateKey string,
	validatorMessagesAtGenesis bool,
	proxyOwnerPrivateKey string,
) (common.Address, error) {
	posValidatorManagerAddress, _, _, err := DeployPoSValidatorManagerV2_0_0Contract(
		rpcURL,
		privateKey,
		validatorMessagesAtGenesis,
	)
	if err != nil {
		return common.Address{}, err
	}
	if _, _, err := SetupGenesisSpecializationProxyImplementation(
		rpcURL,
		proxyOwnerPrivateKey,
		posValidatorManagerAddress,
	); err != nil {
		return common.Address{}, err
	}
	return posValidatorManagerAddress, nil
}

//go:embed smart_contracts/deployed_transparent_proxy_bytecode.txt
var deployedTransparentProxyBytecode []byte

//go:embed smart_contracts/deployed_proxy_admin_bytecode.txt
var deployedProxyAdminBytecode []byte

func AddValidatorTransparentProxyContractToAllocations(
	allocs core.GenesisAlloc,
	proxyManager string,
) {
	if _, found := allocs[common.HexToAddress(proxyManager)]; !found {
		ownerBalance := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1))
		allocs[common.HexToAddress(proxyManager)] = core.GenesisAccount{
			Balance: ownerBalance,
		}
	}
	// proxy admin
	deployedProxyAdmin := common.FromHex(strings.TrimSpace(string(deployedProxyAdminBytecode)))
	allocs[common.HexToAddress(validatormanagerSDK.ValidatorProxyAdminContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedProxyAdmin,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x0"): common.HexToHash(proxyManager),
		},
	}

	// transparent proxy
	deployedTransparentProxy := common.FromHex(strings.TrimSpace(string(deployedTransparentProxyBytecode)))
	allocs[common.HexToAddress(validatormanagerSDK.ValidatorProxyContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedTransparentProxy,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"): common.HexToHash(validatormanagerSDK.ValidatorContractAddress),           // sslot for address of ValidatorManager logic -> bytes32(uint256(keccak256('eip1967.proxy.implementation')) - 1)
			common.HexToHash("0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103"): common.HexToHash(validatormanagerSDK.ValidatorProxyAdminContractAddress), // sslot for address of ProxyAdmin -> bytes32(uint256(keccak256('eip1967.proxy.admin')) - 1)
			// we can omit 3rd sslot for _data, as we initialize ValidatorManager after chain is live
		},
	}
}

func AddSpecializationTransparentProxyContractToAllocations(
	allocs core.GenesisAlloc,
	proxyManager string,
) {
	if _, found := allocs[common.HexToAddress(proxyManager)]; !found {
		ownerBalance := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1))
		allocs[common.HexToAddress(proxyManager)] = core.GenesisAccount{
			Balance: ownerBalance,
		}
	}
	// proxy admin
	deployedProxyAdmin := common.FromHex(strings.TrimSpace(string(deployedProxyAdminBytecode)))
	allocs[common.HexToAddress(validatormanagerSDK.SpecializationProxyAdminContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedProxyAdmin,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x0"): common.HexToHash(proxyManager),
		},
	}

	// transparent proxy
	deployedTransparentProxy := common.FromHex(strings.TrimSpace(string(deployedTransparentProxyBytecode)))
	allocs[common.HexToAddress(validatormanagerSDK.SpecializationProxyContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedTransparentProxy,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"): common.HexToHash(validatormanagerSDK.ValidatorContractAddress),                // sslot for address of ValidatorManager logic -> bytes32(uint256(keccak256('eip1967.proxy.implementation')) - 1)
			common.HexToHash("0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103"): common.HexToHash(validatormanagerSDK.SpecializationProxyAdminContractAddress), // sslot for address of ProxyAdmin -> bytes32(uint256(keccak256('eip1967.proxy.admin')) - 1)
			// we can omit 3rd sslot for _data, as we initialize ValidatorManager after chain is live
		},
	}
}

//go:embed smart_contracts/deployed_example_reward_calculator_bytecode_v2.0.0.txt
var deployedExampleRewardCalculatorV2_0_0Bytecode []byte

func AddRewardCalculatorV2_0_0ToAllocations(
	allocs core.GenesisAlloc,
	rewardBasisPoints uint64,
) {
	deployedExampleRewardCalculatorBytes := common.FromHex(strings.TrimSpace(string(deployedExampleRewardCalculatorV2_0_0Bytecode)))
	allocs[common.HexToAddress(validatormanagerSDK.RewardCalculatorAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedExampleRewardCalculatorBytes,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x0"): common.BigToHash(new(big.Int).SetUint64(rewardBasisPoints)),
		},
	}
}

//go:embed smart_contracts/example_reward_calculator_bytecode_v2.0.0.txt
var exampleRewardCalculatorV2_0_0Bytecode []byte

func DeployRewardCalculatorV2_0_0Contract(
	rpcURL string,
	privateKey string,
	rewardBasisPoints uint64,
) (common.Address, *types.Transaction, *types.Receipt, error) {
	exampleRewardCalculatorBytes := []byte(strings.TrimSpace(string(exampleRewardCalculatorV2_0_0Bytecode)))
	return contract.DeployContract(
		rpcURL,
		privateKey,
		exampleRewardCalculatorBytes,
		"(uint64)",
		rewardBasisPoints,
	)
}

// setups PoA manager after a successful execution of
// ConvertSubnetToL1Tx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func SetupPoA(
	log logging.Logger,
	subnet blockchainSDK.Subnet,
	privateKey string,
	aggregatorLogger logging.Logger,
	v2_0_0 bool,
	signatureAggregatorEndpoint string,
) error {
	return subnet.InitializeProofOfAuthority(
		log,
		privateKey,
		aggregatorLogger,
		v2_0_0,
		signatureAggregatorEndpoint,
	)
}

// setups PoA manager after a successful execution of
// ConvertSubnetToL1Tx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func SetupPoS(
	log logging.Logger,
	subnet blockchainSDK.Subnet,
	privateKey string,
	aggregatorLogger logging.Logger,
	posParams validatormanagerSDK.PoSParams,
	v2_0_0 bool,
	signatureAggregatorEndpoint string,
	nativeMinterPrecompileAdminPrivateKey string,
) error {
	return subnet.InitializeProofOfStake(
		log,
		privateKey,
		aggregatorLogger,
		posParams,
		v2_0_0,
		signatureAggregatorEndpoint,
		nativeMinterPrecompileAdminPrivateKey,
	)
}
