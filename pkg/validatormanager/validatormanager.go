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
	ValidatorContractAddress = "0x5F584C2D56B4c356e7d82EC6129349393dc5df17"
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
// manage validators on [subnetID]
func PoSValidatorManagerInitialize(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID [32]byte,
) (*types.Transaction, *types.Receipt, error) {
	const (
		RewardCalculatorPrecompileAddress = "0x0200000000000000000000000000000000000004"
	)
	var (
		defaultChurnPeriodSeconds     uint64 = 100
		defaultMaximumChurnPercentage uint8  = 10
		defaultMinimumStakeAmount            = big.NewInt(1)
		defaultMaximumStakeAmount            = big.NewInt(100000)
		defaultMinimumStakeDuration   uint64 = 604800 // 1 week in seconds
		defaultMinimumDelegationFee   uint16 = 100    // 1% in basis points
		defaultMaximumStakeMultiplier uint8  = 2
		defaultWeightToValueFactor           = big.NewInt(1)
	)

	type ValidatorManagerSettings struct {
		SubnetID               [32]byte
		ChurnPeriodSeconds     uint64
		MaximumChurnPercentage uint8
	}

	type PoSValidatorManagerSettings struct {
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

	params := PoSValidatorManagerSettings{
		BaseSettings:             baseSettings,
		MinimumStakeAmount:       defaultMinimumStakeAmount,
		MaximumStakeAmount:       defaultMaximumStakeAmount,
		MinimumStakeDuration:     defaultMinimumStakeDuration,
		MinimumDelegationFeeBips: defaultMinimumDelegationFee,
		MaximumStakeMultiplier:   defaultMaximumStakeMultiplier,
		WeightToValueFactor:      defaultWeightToValueFactor,
		RewardCalculator:         common.HexToAddress(RewardCalculatorPrecompileAddress),
	}

	methodSignature := "initialize(((bytes32,uint64,uint8),uint256,uint256,uint64,uint16,uint8,uint256,address))"

	// Call the TxToMethod function
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		managerAddress,
		nil,
		"initialize Native PoS manager",
		errorSignatureToError,
		methodSignature,
		params,
	)
}

// constructs p-chain-validated (signed) subnet conversion warp
// message, to be sent to the validators manager when
// initializing validators set
// the message specifies [subnetID] that is being converted
// together with the validator's manager [managerBlockchainID],
// [managerAddress], and the initial list of [validators]
func GetPChainSubnetConversionWarpMessage(
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

// calls poa manager validators set init method,
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
	managerAddress := common.HexToAddress(ValidatorContractAddress)
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
	subnetConversionSignedMessage, err := GetPChainSubnetConversionWarpMessage(
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
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	tx, _, err := PoSValidatorManagerInitialize(
		rpcURL,
		managerAddress,
		privateKey,
		subnetID,
	)
	if err != nil {
		if !errors.Is(err, errAlreadyInitialized) {
			return evm.TransactionError(tx, err, "failure initializing pos validator manager")
		}
		ux.Logger.PrintToUser("Warning: the PoS contract is already initialized.")
	}
	aggregatorLogLevel, err := logging.ToLevel(aggregatorLogLevelStr)
	if err != nil {
		aggregatorLogLevel = defaultAggregatorLogLevel
	}
	subnetConversionSignedMessage, err := GetPChainSubnetConversionWarpMessage(
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
