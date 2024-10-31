// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
)

const (
	ValidatorContractAddress       = "0x5F584C2D56B4c356e7d82EC6129349393dc5df17"
	ProxyContractAddress           = "0xC0FFEE1234567890aBcDEF1234567890AbCdEf34"
	ProxyAdminContractAddress      = "0xFEEDBEEF0000000000000000000000000000000A"
	ExampleRewardCalculatorAddress = "0xAC1D000000000000000000000000000000000000"
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
	defaultAggregatorLogLevel = logging.Off
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

// initializes contract [managerAddress] at [rpcURL], to
// manage validators on [subnetID], with
// owner given by [ownerAddress]
func PoAValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	ownerAddress common.Address,
) (*types.Transaction, *types.Receipt, error) {
	const (
		defaultChurnPeriodSeconds     = uint64(0)
		defaultMaximumChurnPercentage = uint8(20)
	)
	type Params struct {
		SubnetID               [32]byte
		ChurnPeriodSeconds     uint64
		MaximumChurnPercentage uint8
	}
	params := Params{
		SubnetID:               subnetID,
		ChurnPeriodSeconds:     defaultChurnPeriodSeconds,
		MaximumChurnPercentage: defaultMaximumChurnPercentage,
	}
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		managerAddress,
		nil,
		"initialize PoA manager",
		errorSignatureToError,
		"initialize((bytes32,uint64,uint8),address)",
		params,
		ownerAddress,
	)
}

// initializes contract [managerAddress] at [rpcURL], to
// manage validators on [subnetID] using PoS specific settings
func PoSValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID [32]byte,
	minimumStakeAmount *big.Int,
	maximumStakeAmount *big.Int,
	minimumStakeDuration uint64,
	minimumDelegationFee uint16,
	maximumStakeMultiplier uint8,
	weightToValueFactor *big.Int,
	rewardCalculatorAddress string,
) (*types.Transaction, *types.Receipt, error) {
	var (
		defaultChurnPeriodSeconds     = uint64(0) // no churn period
		defaultMaximumChurnPercentage = uint8(20) // 20% of the validator set can be churned per churn period
	)

	type ValidatorManagerSettings struct {
		SubnetID               [32]byte
		ChurnPeriodSeconds     uint64
		MaximumChurnPercentage uint8
	}

	type NativeTokenValidatorManagerSettings struct {
		BaseSettings             ValidatorManagerSettings
		MinimumStakeAmount       *big.Int
		MaximumStakeAmount       *big.Int
		MinimumStakeDuration     uint64
		MinimumDelegationFeeBips uint16
		MaximumStakeMultiplier   uint8
		WeightToValueFactor      *big.Int
		RewardCalculator         common.Address
	}

	baseSettings := ValidatorManagerSettings{
		SubnetID:               subnetID,
		ChurnPeriodSeconds:     defaultChurnPeriodSeconds,
		MaximumChurnPercentage: defaultMaximumChurnPercentage,
	}

	params := NativeTokenValidatorManagerSettings{
		BaseSettings:             baseSettings,
		MinimumStakeAmount:       minimumStakeAmount,
		MaximumStakeAmount:       maximumStakeAmount,
		MinimumStakeDuration:     minimumStakeDuration,
		MinimumDelegationFeeBips: minimumDelegationFee,
		MaximumStakeMultiplier:   maximumStakeMultiplier,
		WeightToValueFactor:      weightToValueFactor,
		RewardCalculator:         common.HexToAddress(rewardCalculatorAddress),
	}

	return contract.TxToMethod(
		rpcURL,
		privateKey,
		managerAddress,
		nil,
		"initialize Native Token PoS manager",
		errorSignatureToError,
		"initialize(((bytes32,uint64,uint8),uint256,uint256,uint64,uint16,uint8,uint256,address))",
		params,
	)
}

// constructs p-chain-validated (signed) subnet conversion warp
// message, to be sent to the validators manager when
// initializing validators set
// the message specifies [subnetID] that is being converted
// together with the validator's manager [managerBlockchainID],
// [managerAddress], and the initial list of [validators]
func ValidatorManagerGetPChainSubnetConversionWarpMessage(
	network models.Network,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	managerAddress common.Address,
	convertSubnetValidators []*txs.ConvertSubnetValidator,
) (*warp.Message, error) {
	validators := []warpMessage.SubnetConversionValidatorData{}
	for _, convertSubnetValidator := range convertSubnetValidators {
		validators = append(validators, warpMessage.SubnetConversionValidatorData{
			NodeID:       convertSubnetValidator.NodeID[:],
			BLSPublicKey: convertSubnetValidator.Signer.PublicKey,
			Weight:       convertSubnetValidator.Weight,
		})
	}
	subnetConversionData := warpMessage.SubnetConversionData{
		SubnetID:       subnetID,
		ManagerChainID: managerBlockchainID,
		ManagerAddress: managerAddress.Bytes(),
		Validators:     validators,
	}
	subnetConversionID, err := warpMessage.SubnetConversionID(subnetConversionData)
	if err != nil {
		return nil, err
	}
	addressedCallPayload, err := warpMessage.NewSubnetConversion(subnetConversionID)
	if err != nil {
		return nil, err
	}
	subnetConversionAddressedCall, err := warpPayload.NewAddressedCall(
		nil,
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	subnetConversionUnsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		avagoconstants.PlatformChainID,
		subnetConversionAddressedCall.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		network,
		aggregatorLogLevel,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(subnetConversionUnsignedMessage, subnetID[:])
}

// calls validator manager validators set init method,
// passing to it the p-chain signed [subnetConversionSignedMessage]
// so as to verify p-chain already proceesed the associated
// ConvertSubnetTx
func InitializeValidatorsSet(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	convertSubnetValidators []*txs.ConvertSubnetValidator,
	subnetConversionSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
	type InitialValidator struct {
		NodeID       []byte
		BlsPublicKey []byte
		Weight       uint64
	}
	type SubnetConversionData struct {
		SubnetID                     [32]byte
		ValidatorManagerBlockchainID [32]byte
		ValidatorManagerAddress      common.Address
		InitialValidators            []InitialValidator
	}
	validators := []InitialValidator{}
	for _, convertSubnetValidator := range convertSubnetValidators {
		validators = append(validators, InitialValidator{
			NodeID:       convertSubnetValidator.NodeID[:],
			BlsPublicKey: convertSubnetValidator.Signer.PublicKey[:],
			Weight:       convertSubnetValidator.Weight,
		})
	}
	subnetConversionData := SubnetConversionData{
		SubnetID:                     subnetID,
		ValidatorManagerBlockchainID: managerBlockchainID,
		ValidatorManagerAddress:      managerAddress,
		InitialValidators:            validators,
	}
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		privateKey,
		managerAddress,
		subnetConversionSignedMessage,
		big.NewInt(0),
		"initialize validator set",
		errorSignatureToError,
		"initializeValidatorSet((bytes32,bytes32,address,[(bytes,bytes,uint64)]),uint32)",
		subnetConversionData,
		uint32(0),
	)
}

// setups PoA manager after a successful execution of
// ConvertSubnetTx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func SetupPoA(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	privateKey string,
	ownerAddress common.Address,
	convertSubnetValidators []*txs.ConvertSubnetValidator,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogLevelStr string,
) error {
	if err := evm.SetupProposerVM(
		rpcURL,
		privateKey,
	); err != nil {
		return err
	}
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	blockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	managerAddress := common.HexToAddress(ProxyContractAddress)
	tx, _, err := PoAValidatorManagerInitialize(
		rpcURL,
		managerAddress,
		privateKey,
		subnetID,
		ownerAddress,
	)
	if err != nil {
		if !errors.Is(err, errAlreadyInitialized) {
			return evm.TransactionError(tx, err, "failure initializing poa validator manager")
		}
		ux.Logger.PrintToUser("Warning: the PoA contract is already initialized.")
	}
	aggregatorLogLevel, err := logging.ToLevel(aggregatorLogLevelStr)
	if err != nil {
		aggregatorLogLevel = defaultAggregatorLogLevel
	}
	subnetConversionSignedMessage, err := ValidatorManagerGetPChainSubnetConversionWarpMessage(
		network,
		aggregatorLogLevel,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		convertSubnetValidators,
	)
	if err != nil {
		return fmt.Errorf("failure signing subnet conversion warp message: %w", err)
	}
	tx, _, err = InitializeValidatorsSet(
		rpcURL,
		managerAddress,
		privateKey,
		subnetID,
		blockchainID,
		convertSubnetValidators,
		subnetConversionSignedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure initializing validators set on poa manager")
	}
	return nil
}

// setups PoS manager after a successful execution of
// ConvertSubnetTx on P-Chain

func SetupPoS(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	privateKey string,
	convertSubnetValidators []*txs.ConvertSubnetValidator,
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
	if err := evm.SetupProposerVM(
		rpcURL,
		privateKey,
	); err != nil {
		return err
	}
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	blockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	managerAddress := common.HexToAddress(ProxyContractAddress)
	tx, _, err := PoSValidatorManagerInitialize(
		rpcURL,
		managerAddress,
		privateKey,
		subnetID,
		minimumStakeAmount,
		maximumStakeAmount,
		minimumStakeDuration,
		minimumDelegationFee,
		maximumStakeMultiplier,
		weightToValueFactor,
		rewardCalculatorAddress,
	)
	if err != nil {
		if !errors.Is(err, errAlreadyInitialized) {
			return evm.TransactionError(tx, err, "failure initializing native PoS validator manager")
		}
		ux.Logger.PrintToUser("Warning: the PoS contract is already initialized.")
	}
	aggregatorLogLevel, err := logging.ToLevel(aggregatorLogLevelStr)
	if err != nil {
		aggregatorLogLevel = defaultAggregatorLogLevel
	}
	subnetConversionSignedMessage, err := ValidatorManagerGetPChainSubnetConversionWarpMessage(
		network,
		aggregatorLogLevel,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		convertSubnetValidators,
	)
	if err != nil {
		return fmt.Errorf("failure signing subnet conversion warp message: %w", err)
	}
	tx, _, err = InitializeValidatorsSet(
		rpcURL,
		managerAddress,
		privateKey,
		subnetID,
		blockchainID,
		convertSubnetValidators,
		subnetConversionSignedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure initializing validators set on pos manager")
	}
	return nil
}
