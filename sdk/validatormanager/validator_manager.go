// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validatormanager

import (
	"errors"
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
	ValidatorContractAddress = "0x5F584C2D56B4c356e7d82EC6129349393dc5df17"
)

var (
	ErrAlreadyInitialized                  = errors.New("the contract is already initialized")
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
		"InvalidInitialization()":                      ErrAlreadyInitialized,
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
		errorSignatureToError,
		"initialize((bytes32,uint64,uint8),address)",
		params,
		ownerAddress,
	)
}

// GetPoAPChainSubnetConversionWarpMessage constructs p-chain-validated (signed) subnet conversion warp
// message, to be sent to the validators manager when
// initializing validators set
// the message specifies [subnetID] that is being converted
// together with the validator's manager [managerBlockchainID],
// [managerAddress], and the initial list of [validators]
func GetPoAPChainSubnetConversionWarpMessage(
	network models.Network,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	managerBlockchainID ids.ID,
	managerAddress common.Address,
	convertSubnetValidators []*txs.ConvertSubnetToL1Validator,
) (*warp.Message, error) {
	validators := []warpMessage.SubnetToL1ConverstionValidatorData{}
	for _, convertSubnetValidator := range convertSubnetValidators {
		validators = append(validators, warpMessage.SubnetToL1ConverstionValidatorData{
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
func PoAInitializeValidatorsSet(
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
