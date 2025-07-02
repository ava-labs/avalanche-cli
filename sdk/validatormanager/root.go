// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/sdk/constants"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core/types"

	"github.com/ethereum/go-ethereum/common"
)

type ACP99ValidatorManagerSettings struct {
	Admin                  common.Address
	SubnetID               [32]byte
	ChurnPeriodSeconds     uint64
	MaximumChurnPercentage uint8
}

type ValidatorManagerSettings struct {
	SubnetID               [32]byte
	ChurnPeriodSeconds     uint64
	MaximumChurnPercentage uint8
}

type NativeTokenValidatorManagerSettingsV1_0_0 struct {
	BaseSettings             ValidatorManagerSettings
	MinimumStakeAmount       *big.Int
	MaximumStakeAmount       *big.Int
	MinimumStakeDuration     uint64
	MinimumDelegationFeeBips uint16
	MaximumStakeMultiplier   uint8
	WeightToValueFactor      *big.Int
	RewardCalculator         common.Address
	UptimeBlockchainID       [32]byte
}

type NativeTokenValidatorManagerSettingsV2_0_0 struct {
	Manager                  common.Address
	MinimumStakeAmount       *big.Int
	MaximumStakeAmount       *big.Int
	MinimumStakeDuration     uint64
	MinimumDelegationFeeBips uint16
	MaximumStakeMultiplier   uint8
	WeightToValueFactor      *big.Int
	RewardCalculator         common.Address
	UptimeBlockchainID       [32]byte
}

const (
	ValidatorMessagesContractAddress        = "0x9C00629cE712B0255b17A4a657171Acd15720B8C"
	ValidatorContractAddress                = "0x0C0DEBA5E0000000000000000000000000000000"
	ValidatorProxyContractAddress           = "0x0FEEDC0DE0000000000000000000000000000000"
	ValidatorProxyAdminContractAddress      = "0xA0AFFE1234567890aBcDEF1234567890AbCdEf34"
	SpecializationProxyContractAddress      = "0x100C0DE1C0FFEE00000000000000000000000000"
	SpecializationProxyAdminContractAddress = "0x97A35a4A2A8a56256de7A32160819c7B3F4C9DA6"
	RewardCalculatorAddress                 = "0x0DEADC0DE0000000000000000000000000000000"

	DefaultPoSMinimumStakeAmount     = 1
	DefaultPoSMaximumStakeAmount     = 1000
	DefaultPoSMinimumStakeDuration   = 100
	DefaultPoSDMinimumDelegationFee  = 1
	DefaultPoSMaximumStakeMultiplier = 1
	DefaultPoSWeightToValueFactor    = 1
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
	ErrInvalidOwner                        = fmt.Errorf("invalid proxy or validator owner")
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
		"OwnableInvalidOwner(address)":                 ErrInvalidOwner,
		"OwnableUnauthorizedAccount(address)":          ErrUnauthorizedOwner,
	}
)

type PoSParams struct {
	MinimumStakeAmount      *big.Int
	MaximumStakeAmount      *big.Int
	MinimumStakeDuration    uint64
	MinimumDelegationFee    uint16
	MaximumStakeMultiplier  uint8
	WeightToValueFactor     *big.Int
	RewardCalculatorAddress string
	UptimeBlockchainID      ids.ID
}

func (p PoSParams) Verify() error {
	if p.MinimumStakeAmount.Cmp(big.NewInt(0)) < 0 {
		return fmt.Errorf("minimum stake amount cannot be negative")
	}
	if p.MaximumStakeAmount.Cmp(big.NewInt(0)) < 0 {
		return fmt.Errorf("maximum stake amount cannot be negative")
	}
	if p.MaximumStakeAmount.Cmp(p.MinimumStakeAmount) < 0 {
		return fmt.Errorf("maximum stake amount cannot be less than minimum stake amount")
	}
	if p.WeightToValueFactor.Cmp(big.NewInt(0)) < 0 {
		return fmt.Errorf("weight to value factor cannot be negative")
	}
	if p.RewardCalculatorAddress == "" {
		return fmt.Errorf("reward calculator address cannot be empty")
	}
	return nil
}

// GetPChainSubnetToL1ConversionMessage constructs p-chain-validated (signed) subnet conversion warp
// message, to be sent to the validators manager when
// initializing validators set
// the message specifies [subnetID] that is being converted
// together with the validator's manager [managerBlockchainID],
// [managerAddress], and the initial list of [validators]
func GetPChainSubnetToL1ConversionMessage(
	network network.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	managerAddress common.Address,
	convertSubnetValidators []*txs.ConvertSubnetToL1Validator,
) (*warp.Message, error) {
	validators := []warpMessage.SubnetToL1ConversionValidatorData{}
	for _, convertSubnetValidator := range convertSubnetValidators {
		validators = append(validators, warpMessage.SubnetToL1ConversionValidatorData{
			NodeID:       convertSubnetValidator.NodeID[:],
			BLSPublicKey: convertSubnetValidator.Signer.PublicKey,
			Weight:       convertSubnetValidator.Weight,
		})
	}
	subnetConversionData := warpMessage.SubnetToL1ConversionData{
		SubnetID:       subnetID,
		ManagerChainID: managerBlockchainID,
		ManagerAddress: managerAddress.Bytes(),
		Validators:     validators,
	}
	subnetConversionID, err := warpMessage.SubnetToL1ConversionID(subnetConversionData)
	if err != nil {
		return nil, err
	}
	addressedCallPayload, err := warpMessage.NewSubnetToL1Conversion(subnetConversionID)
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
	aggregatorCtx, aggregatorCancel := utils.GetTimedContext(constants.SignatureAggregatorTimeout)
	defer aggregatorCancel()
	signatureAggregator, err := interchain.NewSignatureAggregator(
		aggregatorCtx,
		network,
		aggregatorLogger,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(subnetConversionUnsignedMessage, subnetID[:])
}

// InitializeValidatorsSet calls poa manager validators set init method,
// passing to it the p-chain signed [subnetConversionSignedMessage]
// to verify p-chain already processed the associated ConvertSubnetToL1Tx
func InitializeValidatorsSet(
	rpcURL string,
	managerAddress common.Address,
	privateKey string,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	convertSubnetValidators []*txs.ConvertSubnetToL1Validator,
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
		false,
		common.Address{},
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

// GetValidatorManagerType returns validatormanagertypes.ProofOfAuthority if validator manager is verified to be Proof of Authority
// If validator manager is verified to be Proof of Stake, returns validatormanagertypes.ProofOfStake
// In other cases, returns validatormanagertypes.UndefinedValidatorManagement and the associated error
func GetValidatorManagerType(
	rpcURL string,
	managerAddress common.Address,
) (validatormanagertypes.ValidatorManagementType, error) {
	// Verify that ACP99 validator manager contract is present in the rpc url by calling registeredValidators func in ValidatorManager.sol
	if _, err := validator.GetValidationID(rpcURL, managerAddress, ids.EmptyNodeID); err != nil {
		return validatormanagertypes.UndefinedValidatorManagement, err
	}
	// verify it is PoS
	if _, err := PoSWeightToValue(rpcURL, managerAddress, 0); err != nil {
		// if it is PoA it will return Error: execution reverted
		if strings.Contains(err.Error(), "execution reverted") {
			return validatormanagertypes.ProofOfAuthority, nil
		}
		return validatormanagertypes.UndefinedValidatorManagement, err
	}
	return validatormanagertypes.ProofOfStake, nil
}
