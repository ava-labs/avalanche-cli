// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	"github.com/ava-labs/avalanchego/api/info"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"

	"github.com/ava-labs/avalanchego/ids"
)

const (
	ValidatorContractAddress  = "0xC0DEBA5E0000000000000000000000000000000"
	ProxyContractAddress      = "0xFEEDC0DE0000000000000000000000000000000"
	ProxyAdminContractAddress = "0xC0FFEE1234567890aBcDEF1234567890AbCdEf34"
	RewardCalculatorAddress   = "0xDEADC0DE0000000000000000000000000000000"
)

var (
	ErrDelegatorIneligibleForRewards       = fmt.Errorf("delegator ineligible for rewards")
	ErrInvalidBLSPublicKey                 = fmt.Errorf("invalid BLS public key")
	ErrAlreadyInitialized                  = fmt.Errorf("the contract is already initialized")
	ErrInvalidMaximumChurnPercentage       = fmt.Errorf("unvalid churn percentage")
	ErrInvalidValidationID                 = fmt.Errorf("invalid validation id")
	ErrInvalidValidatorStatus              = fmt.Errorf("invalid validator status")
	ErrMaxChurnRateExceeded                = fmt.Errorf("max churn rate exceeded")
	ErrInvalidInitializationStatus         = fmt.Errorf("validators set already initialized")
	ErrInvalidValidatorManagerBlockchainID = fmt.Errorf("invalid validator manager blockchain ID")
	ErrInvalidValidatorManagerAddress      = fmt.Errorf("invalid validator manager address")
	ErrNodeAlreadyRegistered               = fmt.Errorf("node already registered")
	ErrInvalidSubnetConversionID           = fmt.Errorf("invalid subnet conversion id")
	ErrInvalidRegistrationExpiry           = fmt.Errorf("invalid registration expiry")
	ErrInvalidBLSKeyLength                 = fmt.Errorf("invalid BLS key length")
	ErrInvalidNodeID                       = fmt.Errorf("invalid node id")
	ErrInvalidWarpMessage                  = fmt.Errorf("invalid warp message")
	ErrInvalidWarpSourceChainID            = fmt.Errorf("invalid wapr source chain ID")
	ErrInvalidWarpOriginSenderAddress      = fmt.Errorf("invalid warp origin sender address")
	ErrInvalidCodecID                      = fmt.Errorf("invalid codec ID")
	ErrInvalidConversionID                 = fmt.Errorf("invalid conversion ID")
	ErrInvalidDelegationFee                = fmt.Errorf("invalid delegation fee")
	ErrInvalidDelegationID                 = fmt.Errorf("invalid delegation ID")
	ErrInvalidDelegatorStatus              = fmt.Errorf("invalid delegator status")
	ErrInvalidMessageLength                = fmt.Errorf("invalid message length")
	ErrInvalidMessageType                  = fmt.Errorf("invalid message type")
	ErrInvalidMinStakeDuration             = fmt.Errorf("invalid min stake duration")
	ErrInvalidNonce                        = fmt.Errorf("invalid nonce")
	ErrInvalidPChainOwnerThreshold         = fmt.Errorf("invalid pchain owner threshold")
	ErrInvalidStakeAmount                  = fmt.Errorf("invalid stake amount")
	ErrInvalidStakeMultiplier              = fmt.Errorf("invalid stake multiplier")
	ErrInvalidTokenAddress                 = fmt.Errorf("invalid token address")
	ErrInvalidTotalWeight                  = fmt.Errorf("invalid total weight")
	ErrMaxWeightExceeded                   = fmt.Errorf("max weight exceeded")
	ErrMinStakeDurationNotPassed           = fmt.Errorf("min stake duration not passed")
	ErrPChainOwnerAddressesNotSorted       = fmt.Errorf("pchain owner addresses not sorted")
	ErrUnauthorizedOwner                   = fmt.Errorf("unauthorized owner")
	ErrUnexpectedRegistrationStatus        = fmt.Errorf("unexpected registration status")
	ErrValidatorIneligibleForRewards       = fmt.Errorf("validator ineligible for rewards")
	ErrValidatorNotPoS                     = fmt.Errorf("validator not PoS")
	ErrZeroWeightToValueFactor             = fmt.Errorf("zero weight to value factor")
	ErrorSignatureToError                  = map[string]error{
		"InvalidInitialization()":                      ErrAlreadyInitialized,
		"InvalidMaximumChurnPercentage(uint8)":         ErrInvalidMaximumChurnPercentage,
		"InvalidValidationID(bytes32)":                 ErrInvalidValidationID,
		"InvalidValidatorStatus(uint8)":                ErrInvalidValidatorStatus,
		"MaxChurnRateExceeded(uint64)":                 ErrMaxChurnRateExceeded,
		"InvalidInitializationStatus()":                ErrInvalidInitializationStatus,
		"InvalidValidatorManagerBlockchainID(bytes32)": ErrInvalidValidatorManagerBlockchainID,
		"InvalidValidatorManagerAddress(address)":      ErrInvalidValidatorManagerAddress,
		"NodeAlreadyRegistered(bytes)":                 ErrNodeAlreadyRegistered,
		"InvalidSubnetConversionID(bytes32,bytes32)":   ErrInvalidSubnetConversionID,
		"InvalidRegistrationExpiry(uint64)":            ErrInvalidRegistrationExpiry,
		"InvalidBLSKeyLength(uint256)":                 ErrInvalidBLSKeyLength,
		"InvalidNodeID(bytes)":                         ErrInvalidNodeID,
		"InvalidWarpMessage()":                         ErrInvalidWarpMessage,
		"InvalidWarpSourceChainID(bytes32)":            ErrInvalidWarpSourceChainID,
		"InvalidWarpOriginSenderAddress(address)":      ErrInvalidWarpOriginSenderAddress,
		"DelegatorIneligibleForRewards(bytes32)":       ErrDelegatorIneligibleForRewards,
		"InvalidBLSPublicKey()":                        ErrInvalidBLSPublicKey,
		"InvalidCodecID(uint32)":                       ErrInvalidCodecID,
		"InvalidConversionID(bytes32,bytes32)":         ErrInvalidConversionID,
		"InvalidDelegationFee(uint16)":                 ErrInvalidDelegationFee,
		"InvalidDelegationID(bytes32)":                 ErrInvalidDelegationID,
		"InvalidDelegatorStatus(DelegatorStatus)":      ErrInvalidDelegatorStatus,
		"InvalidMessageLength(uint32,uint32)":          ErrInvalidMessageLength,
		"InvalidMessageType()":                         ErrInvalidMessageType,
		"InvalidMinStakeDuration(uint64)":              ErrInvalidMinStakeDuration,
		"InvalidNonce(uint64)":                         ErrInvalidNonce,
		"InvalidPChainOwnerThreshold(uint256,uint256)": ErrInvalidPChainOwnerThreshold,
		"InvalidStakeAmount(uint256)":                  ErrInvalidStakeAmount,
		"InvalidStakeMultiplier(uint8)":                ErrInvalidStakeMultiplier,
		"InvalidTokenAddress(address)":                 ErrInvalidTokenAddress,
		"InvalidTotalWeight(uint256)":                  ErrInvalidTotalWeight,
		"MaxWeightExceeded(uint64)":                    ErrMaxWeightExceeded,
		"MinStakeDurationNotPassed(uint64)":            ErrMinStakeDurationNotPassed,
		"PChainOwnerAddressesNotSorted()":              ErrPChainOwnerAddressesNotSorted,
		"UnauthorizedOwner(address)":                   ErrUnauthorizedOwner,
		"UnexpectedRegistrationStatus(bool)":           ErrUnexpectedRegistrationStatus,
		"ValidatorIneligibleForRewards(bytes32)":       ErrValidatorIneligibleForRewards,
		"ValidatorNotPoS(bytes32)":                     ErrValidatorNotPoS,
		"ZeroWeightToValueFactor()":                    ErrZeroWeightToValueFactor,
	}
)

// PoAValidatorManagerInitialize initializes contract [managerAddress] at [rpcURL], to
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
		ErrorSignatureToError,
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
		ErrorSignatureToError,
		"initialize(((bytes32,uint64,uint8),uint256,uint256,uint64,uint16,uint8,uint256,address))",
		params,
	)
}

// GetPChainSubnetConversionWarpMessage constructs p-chain-validated (signed) subnet conversion warp
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

// PoAInitializeValidatorsSet calls poa manager validators set init method,
// passing to it the p-chain signed [subnetConversionSignedMessage]
// to verify p-chain already processed the associated ConvertSubnetTx
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
		ErrorSignatureToError,
		"initializeValidatorSet((bytes32,bytes32,address,[(bytes,bytes,uint64)]),uint32)",
		subnetConversionData,
		uint32(0),
	)
}