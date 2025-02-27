// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	"github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager"
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

func InitializeValidatorRegistrationPoSNative(
	rpcURL string,
	managerAddress common.Address,
	managerOwnerPrivateKey string,
	nodeID ids.NodeID,
	blsPublicKey []byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	delegationFeeBips uint16,
	minStakeDuration time.Duration,
	stakeAmount *big.Int,
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
		stakeAmount,
		"initialize validator registration with stake",
		validatormanager.ErrorSignatureToError,
		"initializeValidatorRegistration((bytes,bytes,uint64,(uint32,[address]),(uint32,[address])),uint16,uint64)",
		validatorRegistrationInput,
		delegationFeeBips,
		uint64(minStakeDuration.Seconds()),
	)
}

// step 1 of flow for adding a new validator
func InitializeValidatorRegistrationPoA(
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
		validatormanager.ErrorSignatureToError,
		"initializeValidatorRegistration((bytes,bytes,uint64,(uint32,[address]),(uint32,[address])),uint64)",
		validatorRegistrationInput,
		weight,
	)
}

func GetSubnetValidatorRegistrationMessage(
	ctx context.Context,
	rpcURL string,
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorAllowPrivateIPs bool,
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
	alreadyInitialized bool,
) (*warp.Message, ids.ID, error) {
	var (
		registerSubnetValidatorUnsignedMessage *warp.UnsignedMessage
		validationID                           ids.ID
		err                                    error
	)
	if alreadyInitialized {
		validationID, err = validator.GetRegisteredValidator(
			rpcURL,
			managerAddress,
			nodeID,
		)
		if err != nil {
			return nil, ids.Empty, err
		}
		unsignedMessageBytes, err := GetRegistrationMessage(
			rpcURL,
			validationID,
		)
		if err != nil {
			return nil, ids.Empty, err
		}
		registerSubnetValidatorUnsignedMessage, err = warp.ParseUnsignedMessage(unsignedMessageBytes)
		if err != nil {
			return nil, ids.Empty, err
		}
	} else {
		addressedCallPayload, err := warpMessage.NewRegisterL1Validator(
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
		validationID = addressedCallPayload.ValidationID()
		registerSubnetValidatorAddressedCall, err := warpPayload.NewAddressedCall(
			managerAddress.Bytes(),
			addressedCallPayload.Bytes(),
		)
		if err != nil {
			return nil, ids.Empty, err
		}
		registerSubnetValidatorUnsignedMessage, err = warp.NewUnsignedMessage(
			network.ID,
			blockchainID,
			registerSubnetValidatorAddressedCall.Bytes(),
		)
		if err != nil {
			return nil, ids.Empty, err
		}
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		ctx,
		network,
		aggregatorLogger,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorAllowPrivateIPs,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	signedMessage, err := signatureAggregator.Sign(registerSubnetValidatorUnsignedMessage, nil)
	return signedMessage, validationID, err
}

func PoSWeightToValue(
	rpcURL string,
	managerAddress common.Address,
	weight uint64,
) (*big.Int, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"weightToValue(uint64)->(uint256)",
		weight,
	)
	if err != nil {
		return nil, err
	}
	value, b := out[0].(*big.Int)
	if !b {
		return nil, fmt.Errorf("error at weightToValue, expected *big.Int, got %T", out[0])
	}
	return value, nil
}

func GetPChainSubnetValidatorRegistrationWarpMessage(
	ctx context.Context,
	network models.Network,
	rpcURL string,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorAllowPrivateIPs bool,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	validationID ids.ID,
	registered bool,
) (*warp.Message, error) {
	addressedCallPayload, err := warpMessage.NewL1ValidatorRegistration(validationID, registered)
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
		ctx,
		network,
		aggregatorLogger,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorAllowPrivateIPs,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	var justificationBytes []byte
	if !registered {
		justificationBytes, err = GetRegistrationJustification(rpcURL, validationID, subnetID)
		if err != nil {
			return nil, err
		}
	}
	return signatureAggregator.Sign(subnetConversionUnsignedMessage, justificationBytes)
}

// last step of flow for adding a new validator
func CompleteValidatorRegistration(
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
		"complete validator registration",
		validatormanager.ErrorSignatureToError,
		"completeValidatorRegistration(uint32)",
		uint32(0),
	)
}

func InitValidatorRegistration(
	ctx context.Context,
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
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	initWithPos bool,
	delegationFee uint16,
	stakeDuration time.Duration,
	validatorManagerAddressStr string,
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
	managerAddress := common.HexToAddress(validatorManagerAddressStr)
	alreadyInitialized := false
	if initWithPos {
		stakeAmount, err := PoSWeightToValue(
			rpcURL,
			managerAddress,
			weight,
		)
		if err != nil {
			return nil, ids.Empty, fmt.Errorf("failure obtaining value from weight: %w", err)
		}
		ux.Logger.PrintLineSeparator()
		ux.Logger.PrintToUser("Initializing validator registration with PoS validator manager")
		ux.Logger.PrintToUser("Using RPC URL: %s", rpcURL)
		ux.Logger.PrintToUser("NodeID: %s staking %s tokens", nodeID.String(), stakeAmount)
		ux.Logger.PrintLineSeparator()
		tx, _, err := InitializeValidatorRegistrationPoSNative(
			rpcURL,
			managerAddress,
			ownerPrivateKey,
			nodeID,
			blsPublicKey,
			expiry,
			balanceOwners,
			disableOwners,
			delegationFee,
			stakeDuration,
			stakeAmount,
		)
		if err != nil {
			if !errors.Is(err, validatormanager.ErrNodeAlreadyRegistered) {
				return nil, ids.Empty, evm.TransactionError(tx, err, "failure initializing validator registration")
			}
			ux.Logger.PrintToUser(logging.LightBlue.Wrap("The validator registration was already initialized. Proceeding to the next step"))
			alreadyInitialized = true
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Validator staked amount: %d", stakeAmount))
	} else {
		managerAddress = common.HexToAddress(validatorManagerAddressStr)
		tx, _, err := InitializeValidatorRegistrationPoA(
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
			if !errors.Is(err, validatormanager.ErrNodeAlreadyRegistered) {
				return nil, ids.Empty, evm.TransactionError(tx, err, "failure initializing validator registration")
			}
			ux.Logger.PrintToUser(logging.LightBlue.Wrap("The validator registration was already initialized. Proceeding to the next step"))
			alreadyInitialized = true
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Validator weight: %d", weight))
	}

	return GetSubnetValidatorRegistrationMessage(
		ctx,
		rpcURL,
		network,
		aggregatorLogger,
		0,
		aggregatorAllowPrivatePeers,
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
		alreadyInitialized,
	)
}

func FinishValidatorRegistration(
	ctx context.Context,
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	privateKey string,
	validationID ids.ID,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	validatorManagerAddressStr string,
) error {
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return err
	}
	managerAddress := common.HexToAddress(validatorManagerAddressStr)
	signedMessage, err := GetPChainSubnetValidatorRegistrationWarpMessage(
		ctx,
		network,
		rpcURL,
		aggregatorLogger,
		0,
		aggregatorAllowPrivatePeers,
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
		ux.Logger.RedXToUser("failure setting proposer VM on L1: %w", err)
	}
	tx, _, err := CompleteValidatorRegistration(
		rpcURL,
		managerAddress,
		privateKey,
		signedMessage,
	)
	if err != nil {
		if !errors.Is(err, validatormanager.ErrInvalidValidationID) {
			return evm.TransactionError(tx, err, "failure completing validator registration")
		} else {
			return fmt.Errorf("the Validator was already fully registered on the Manager")
		}
	}
	return nil
}

func GetRegistrationMessage(
	rpcURL string,
	validationID ids.ID,
) ([]byte, error) {
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
					reg, err := warpMessage.ParseRegisterL1Validator(addressedCall.Payload)
					if err == nil {
						if reg.ValidationID() == validationID {
							return msg.Bytes(), nil
						}
					}
				}
			}
		}
	}
	return nil, fmt.Errorf("validation id %s not found on warp events", validationID)
}

func GetRegistrationJustification(
	rpcURL string,
	validationID ids.ID,
	subnetID ids.ID,
) ([]byte, error) {
	const numBootstrapValidatorsToSearch = 100
	for validationIndex := uint32(0); validationIndex < numBootstrapValidatorsToSearch; validationIndex++ {
		bootstrapValidationID := subnetID.Append(validationIndex)
		if bootstrapValidationID == validationID {
			justification := platformvm.L1ValidatorRegistrationJustification{
				Preimage: &platformvm.L1ValidatorRegistrationJustification_ConvertSubnetToL1TxData{
					ConvertSubnetToL1TxData: &platformvm.SubnetIDIndex{
						SubnetId: subnetID[:],
						Index:    validationIndex,
					},
				},
			}
			return proto.Marshal(&justification)
		}
	}
	msg, err := GetRegistrationMessage(
		rpcURL,
		validationID,
	)
	if err != nil {
		return nil, err
	}
	parsed, err := warp.ParseUnsignedMessage(msg)
	if err != nil {
		return nil, err
	}
	payload := parsed.Payload
	addressedCall, err := warpPayload.ParseAddressedCall(payload)
	if err != nil {
		return nil, err
	}
	justification := platformvm.L1ValidatorRegistrationJustification{
		Preimage: &platformvm.L1ValidatorRegistrationJustification_RegisterL1ValidatorMessage{
			RegisterL1ValidatorMessage: addressedCall.Payload,
		},
	}
	return proto.Marshal(&justification)
}
