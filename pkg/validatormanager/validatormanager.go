// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
)

const (
	ValidatorContractAddress       = "0xC0DEBA5E0000000000000000000000000000000"
	ProxyContractAddress           = "0xFEEDC0DE0000000000000000000000000000000"
	ProxyAdminContractAddress      = "0xC0FFEE1234567890aBcDEF1234567890AbCdEf34"
	ExampleRewardCalculatorAddress = "0xDEADC0DE0000000000000000000000000000000"

	defaultAggregatorLogLevel = logging.Off
)

var (
	errAlreadyInitialized                  = errors.New("the contract is already initialized")
	errInvalidMaximumChurnPercentage       = fmt.Errorf("unvalid churn percentage")
	errInvalidValidationID                 = fmt.Errorf("invalid validation id")
	errInvalidValidatorStatus              = fmt.Errorf("invalid validator status")
	errMaxChurnRateExceeded                = fmt.Errorf("max churn rate exceeded")
	errInvalidInitializationStatus         = fmt.Errorf("validators set already initialized")
	errInvalidValidatorManagerBlockchainID = fmt.Errorf("invalid validator manager blockchain ID")
	errInvalidValidatorManagerAddress      = fmt.Errorf("invalid validator manager address")
	errNodeAlreadyRegistered               = fmt.Errorf("node already registered")
	errInvalidSubnetConversionID           = fmt.Errorf("invalid subnet conversion id")
	errInvalidRegistrationExpiry           = fmt.Errorf("invalid registration expiry")
	errInvalidBLSKeyLength                 = fmt.Errorf("invalid BLS key length")
	errInvalidNodeID                       = fmt.Errorf("invalid node id")
	errInvalidWarpMessage                  = fmt.Errorf("invalid warp message")
	errInvalidWarpSourceChainID            = fmt.Errorf("invalid wapr source chain ID")
	errInvalidWarpOriginSenderAddress      = fmt.Errorf("invalid warp origin sender address")
	errorSignatureToError                  = map[string]error{
		"InvalidInitialization()":                      errAlreadyInitialized,
		"InvalidMaximumChurnPercentage(uint8)":         errInvalidMaximumChurnPercentage,
		"InvalidValidationID(bytes32)":                 errInvalidValidationID,
		"InvalidValidatorStatus(uint8)":                errInvalidValidatorStatus,
		"MaxChurnRateExceeded(uint64)":                 errMaxChurnRateExceeded,
		"InvalidInitializationStatus()":                errInvalidInitializationStatus,
		"InvalidValidatorManagerBlockchainID(bytes32)": errInvalidValidatorManagerBlockchainID,
		"InvalidValidatorManagerAddress(address)":      errInvalidValidatorManagerAddress,
		"NodeAlreadyRegistered(bytes)":                 errNodeAlreadyRegistered,
		"InvalidSubnetConversionID(bytes32,bytes32)":   errInvalidSubnetConversionID,
		"InvalidRegistrationExpiry(uint64)":            errInvalidRegistrationExpiry,
		"InvalidBLSKeyLength(uint256)":                 errInvalidBLSKeyLength,
		"InvalidNodeID(bytes)":                         errInvalidNodeID,
		"InvalidWarpMessage()":                         errInvalidWarpMessage,
		"InvalidWarpSourceChainID(bytes32)":            errInvalidWarpSourceChainID,
		"InvalidWarpOriginSenderAddress(address)":      errInvalidWarpOriginSenderAddress,
	}
)

//go:embed deployed_poa_validator_manager_bytecode.txt
var deployedPoAValidatorManagerBytecode []byte

func AddPoAValidatorManagerContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedPoaValidatorManagerBytes := common.FromHex(strings.TrimSpace(string(deployedPoAValidatorManagerBytecode)))
	allocs[common.HexToAddress(ValidatorContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedPoaValidatorManagerBytes,
		Nonce:   1,
	}
}

//go:embed deployed_native_pos_validator_manager_bytecode.txt
var deployedPoSValidatorManagerBytecode []byte

func AddPoSValidatorManagerContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedPoSValidatorManagerBytes := common.FromHex(strings.TrimSpace(string(deployedPoSValidatorManagerBytecode)))
	allocs[common.HexToAddress(ValidatorContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedPoSValidatorManagerBytes,
		Nonce:   1,
	}
}

//go:embed deployed_transparent_proxy_bytecode.txt
var deployedTransparentProxyBytecode []byte

//go:embed deployed_proxy_admin_bytecode.txt
var deployedProxyAdminBytecode []byte

func AddTransparentProxyContractToAllocations(
	allocs core.GenesisAlloc,
	proxyManager string,
) {
	// proxy admin
	deployedProxyAdmin := common.FromHex(strings.TrimSpace(string(deployedProxyAdminBytecode)))
	allocs[common.HexToAddress(ProxyAdminContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedProxyAdmin,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x0"): common.HexToHash(proxyManager),
		},
	}

	// transparent proxy
	deployedTransparentProxy := common.FromHex(strings.TrimSpace(string(deployedTransparentProxyBytecode)))
	allocs[common.HexToAddress(ProxyContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedTransparentProxy,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"): common.HexToHash(ValidatorContractAddress),  // sslot for address of ValidatorManager logic -> bytes32(uint256(keccak256('eip1967.proxy.implementation')) - 1)
			common.HexToHash("0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103"): common.HexToHash(ProxyAdminContractAddress), // sslot for address of ProxyAdmin -> bytes32(uint256(keccak256('eip1967.proxy.admin')) - 1)
			// we can omit 3rd sslot for _data, as we initialize ValidatorManager after chain is live
		},
	}
}

//go:embed deployed_example_reward_calculator_bytecode.txt
var deployedRewardCalculatorBytecode []byte

func AddRewardCalculatorToAllocations(
	allocs core.GenesisAlloc,
	rewardBasisPoints uint64,
) {
	deployedRewardCalculatorBytes := common.FromHex(strings.TrimSpace(string(deployedRewardCalculatorBytecode)))
	allocs[common.HexToAddress(ExampleRewardCalculatorAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedRewardCalculatorBytes,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x0"): common.BigToHash(new(big.Int).SetUint64(rewardBasisPoints)),
		},
	}
}

// setups PoA manager after a successful execution of
// ConvertSubnetTx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func SetupPoA(
	subnet blockchainSDK.Subnet,
	network models.Network,
	privateKey string,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogLevelStr string,
) error {
	aggregatorLogLevel, err := logging.ToLevel(aggregatorLogLevelStr)
	if err != nil {
		aggregatorLogLevel = defaultAggregatorLogLevel
	}
	return subnet.InitializeProofOfAuthority(network, privateKey, aggregatorExtraPeerEndpoints, aggregatorLogLevel)
}

// setups PoA manager after a successful execution of
// ConvertSubnetTx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func SetupPoS(
	subnet blockchainSDK.Subnet,
	network models.Network,
	privateKey string,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogLevelStr string,
	minimumStakeAmount *big.Int,
	maximumStakeAmount *big.Int,
	minimumStakeDuration uint64,
	minimumDelegationFee uint16,
	maximumStakeMultiplier uint8,
	weightToValueFactor *big.Int,
	rewardCalculatorAddress string,
) error {
	aggregatorLogLevel, err := logging.ToLevel(aggregatorLogLevelStr)
	if err != nil {
		aggregatorLogLevel = defaultAggregatorLogLevel
	}
	return subnet.InitializeProofOfStake(network,
		privateKey,
		aggregatorExtraPeerEndpoints,
		aggregatorLogLevel,
		minimumStakeAmount,
		maximumStakeAmount,
		minimumStakeDuration,
		minimumDelegationFee,
		maximumStakeMultiplier,
		weightToValueFactor,
		rewardCalculatorAddress,
	)
}
