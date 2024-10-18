// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/proto/pb/platformvm"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/interfaces"
	subnetEvmWarp "github.com/ava-labs/subnet-evm/precompile/contracts/warp"
	"github.com/ethereum/go-ethereum/common"
	"google.golang.org/protobuf/proto"
)

// step 1 of flow for adding a new validator
func PoAValidatorManagerInitializeValidatorRegistration(
	rpcURL string,
	managerAddress common.Address,
	managerOwnerPrivateKey string,
	nodeID ids.NodeID,
	blsPublicKey []byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	weight uint64,
) (*types.Transaction, *types.Receipt, error) {
	type PChainOwner struct {
		Threshold uint32
		Addresses []common.Address
	}
	type ValidatorRegistrationInput struct {
		NodeID                []byte
		BlsPublicKey          []byte
		RegistrationExpiry    uint64
		RemainingBalanceOwner PChainOwner
		DisableOwner          PChainOwner
	}
	balanceOwnersAux := PChainOwner{
		Threshold: balanceOwners.Threshold,
		Addresses: utils.Map(balanceOwners.Addresses, func(addr ids.ShortID) common.Address {
			return common.BytesToAddress(addr[:])
		}),
	}
	disableOwnersAux := PChainOwner{
		Threshold: disableOwners.Threshold,
		Addresses: utils.Map(disableOwners.Addresses, func(addr ids.ShortID) common.Address {
			return common.BytesToAddress(addr[:])
		}),
	}
	validatorRegistrationInput := ValidatorRegistrationInput{
		NodeID:                nodeID[:],
		BlsPublicKey:          blsPublicKey,
		RegistrationExpiry:    expiry,
		RemainingBalanceOwner: balanceOwnersAux,
		DisableOwner:          disableOwnersAux,
	}
	return contract.TxToMethod(
		rpcURL,
		managerOwnerPrivateKey,
		managerAddress,
		big.NewInt(0),
		"initialize validator registration",
		errorSignatureToError,
		"initializeValidatorRegistration((bytes,bytes,uint64,(uint32,[address]),(uint32,[address])),uint64)",
		validatorRegistrationInput,
		weight,
	)
}

func PoaValidatorManagerGetSubnetValidatorRegistrationMessage(
	network models.Network,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	blockchainID ids.ID,
	managerAddress common.Address,
	nodeID ids.NodeID,
	blsPublicKey [48]byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	weight uint64,
) (*warp.Message, ids.ID, error) {
	addressedCallPayload, err := warpMessage.NewRegisterSubnetValidator(
		subnetID,
		nodeID,
		blsPublicKey,
		expiry,
		balanceOwners,
		disableOwners,
		weight,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	validationID := addressedCallPayload.ValidationID()
	registerSubnetValidatorAddressedCall, err := warpPayload.NewAddressedCall(
		managerAddress.Bytes(),
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	registerSubnetValidatorUnsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		blockchainID,
		registerSubnetValidatorAddressedCall.Bytes(),
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		network,
		aggregatorLogLevel,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	signedMessage, err := signatureAggregator.Sign(registerSubnetValidatorUnsignedMessage, nil)
	return signedMessage, validationID, err
}

func GetRegisteredValidator(
	rpcURL string,
	managerAddress common.Address,
	nodeID ids.NodeID,
) (ids.ID, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"registeredValidators(bytes)->(bytes32)",
		nodeID[:],
	)
	if err != nil {
		return ids.Empty, err
	}
	validatorID, b := out[0].([32]byte)
	if !b {
		return ids.Empty, fmt.Errorf("error at registeredValidators call, expected [32]byte, got %T", out[0])
	}
	return validatorID, nil
}

func PoaValidatorManagerGetPChainSubnetValidatorRegistrationWarpMessage(
	network models.Network,
	rpcURL string,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	validationID ids.ID,
	registered bool,
) (*warp.Message, error) {
	addressedCallPayload, err := warpMessage.NewSubnetValidatorRegistration(validationID, registered)
	if err != nil {
		return nil, err
	}
	subnetValidatorRegistrationAddressedCall, err := warpPayload.NewAddressedCall(
		nil,
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	subnetConversionUnsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		avagoconstants.PlatformChainID,
		subnetValidatorRegistrationAddressedCall.Bytes(),
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
	var justificationBytes []byte
	if !registered {
		justificationBytes, err = GetRegistrationMessage(rpcURL, validationID, subnetID)
		if err != nil {
			return nil, err
		}
	}
	return signatureAggregator.Sign(subnetConversionUnsignedMessage, justificationBytes)
}

// last step of flow for adding a new validator
func PoAValidatorManagerCompleteValidatorRegistration(
	rpcURL string,
	managerAddress common.Address,
	privateKey string, // not need to be owner atm
	subnetValidatorRegistrationSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		privateKey,
		managerAddress,
		subnetValidatorRegistrationSignedMessage,
		big.NewInt(0),
		"complete poa validator registration",
		errorSignatureToError,
		"completeValidatorRegistration(uint32)",
		uint32(0),
	)
}

func InitValidatorRegistration(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	ownerPrivateKey string,
	nodeID ids.NodeID,
	blsPublicKey []byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	weight uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogLevelStr string,
) (*warp.Message, ids.ID, error) {
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	blockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	tx, _, err := PoAValidatorManagerInitializeValidatorRegistration(
		rpcURL,
		managerAddress,
		ownerPrivateKey,
		nodeID,
		blsPublicKey,
		expiry,
		balanceOwners,
		disableOwners,
		weight,
	)
	if err != nil {
		if !errors.Is(err, errNodeAlreadyRegistered) {
			return nil, ids.Empty, evm.TransactionError(tx, err, "failure initializing validator registration")
		}
		ux.Logger.PrintToUser("Warning: the validator registration was already initialized. Proceeding to the next step")
	}
	aggregatorLogLevel, err := logging.ToLevel(aggregatorLogLevelStr)
	if err != nil {
		aggregatorLogLevel = defaultAggregatorLogLevel
	}
	return PoaValidatorManagerGetSubnetValidatorRegistrationMessage(
		network,
		aggregatorLogLevel,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		nodeID,
		[48]byte(blsPublicKey),
		expiry,
		balanceOwners,
		disableOwners,
		weight,
	)
}

func FinishValidatorRegistration(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	privateKey string,
	validationID ids.ID,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogLevelStr string,
) error {
	managerAddress := common.HexToAddress(ValidatorContractAddress)
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	aggregatorLogLevel, err := logging.ToLevel(aggregatorLogLevelStr)
	if err != nil {
		aggregatorLogLevel = defaultAggregatorLogLevel
	}
	signedMessage, err := PoaValidatorManagerGetPChainSubnetValidatorRegistrationWarpMessage(
		network,
		rpcURL,
		aggregatorLogLevel,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		validationID,
		true,
	)
	if err != nil {
		return err
	}
	if err := evm.SetupProposerVM(
		rpcURL,
		privateKey,
	); err != nil {
		return err
	}
	tx, _, err := PoAValidatorManagerCompleteValidatorRegistration(
		rpcURL,
		managerAddress,
		privateKey,
		signedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure completing validator registration")
	}
	return nil
}

func GetRegistrationMessage(
	rpcURL string,
	validationID ids.ID,
	subnetID ids.ID,
) ([]byte, error) {
	const numBootstrapValidatorsToSearch = 100
	for validationIndex := uint32(0); validationIndex < numBootstrapValidatorsToSearch; validationIndex++ {
		bootstrapValidationID := subnetID.Append(validationIndex)
		if bootstrapValidationID == validationID {
			justification := platformvm.SubnetValidatorRegistrationJustification{
				Preimage: &platformvm.SubnetValidatorRegistrationJustification_ConvertSubnetTxData{
					ConvertSubnetTxData: &platformvm.SubnetIDIndex{
						SubnetId: subnetID[:],
						Index:    validationIndex,
					},
				},
			}
			return proto.Marshal(&justification)
		}
	}
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	height, err := client.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}
	for blockNumber := uint64(0); blockNumber <= height; blockNumber++ {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
		if err != nil {
			return nil, err
		}
		blockHash := block.Hash()
		logs, err := client.FilterLogs(ctx, interfaces.FilterQuery{
			BlockHash: &blockHash,
			Addresses: []common.Address{subnetEvmWarp.Module.Address},
		})
		if err != nil {
			return nil, err
		}
		for _, txLog := range logs {
			msg, err := subnetEvmWarp.UnpackSendWarpEventDataToMessage(txLog.Data)
			if err == nil {
				payload := msg.Payload
				addressedCall, err := warpPayload.ParseAddressedCall(payload)
				if err == nil {
					reg, err := warpMessage.ParseRegisterSubnetValidator(addressedCall.Payload)
					if err == nil {
						if reg.ValidationID() == validationID {
							justification := platformvm.SubnetValidatorRegistrationJustification{
								Preimage: &platformvm.SubnetValidatorRegistrationJustification_RegisterSubnetValidatorMessage{
									RegisterSubnetValidatorMessage: addressedCall.Payload,
								},
							}
							return proto.Marshal(&justification)
						}
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("validation id %s not found on warp events", validationID)
}
