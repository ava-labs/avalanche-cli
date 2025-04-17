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
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
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
		false,
		common.Address{},
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
	generateRawTxOnly bool,
	managerOwnerAddress common.Address,
	managerOwnerPrivateKey string,
	nodeID ids.NodeID,
	blsPublicKey []byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	weight uint64,
	useACP99 bool,
) (*types.Transaction, *types.Receipt, error) {
	type PChainOwner struct {
		Threshold uint32
		Addresses []common.Address
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
	if useACP99 {
		return contract.TxToMethod(
			rpcURL,
			generateRawTxOnly,
			managerOwnerAddress,
			managerOwnerPrivateKey,
			managerAddress,
			big.NewInt(0),
			"initialize validator registration",
			validatormanager.ErrorSignatureToError,
			"initiateValidatorRegistration(bytes,bytes,uint64,(uint32,[address]),(uint32,[address]),uint64)",
			nodeID[:],
			blsPublicKey,
			expiry,
			balanceOwnersAux,
			disableOwnersAux,
			weight,
		)
	}
	type ValidatorRegistrationInput struct {
		NodeID                []byte
		BlsPublicKey          []byte
		RegistrationExpiry    uint64
		RemainingBalanceOwner PChainOwner
		DisableOwner          PChainOwner
	}
	return contract.TxToMethod(
		rpcURL,
		generateRawTxOnly,
		managerOwnerAddress,
		managerOwnerPrivateKey,
		managerAddress,
		big.NewInt(0),
		"initialize validator registration",
		validatormanager.ErrorSignatureToError,
		"initializeValidatorRegistration((bytes,bytes,uint64,(uint32,[address]),(uint32,[address])),uint64)",
		ValidatorRegistrationInput{
			NodeID:                nodeID[:],
			BlsPublicKey:          blsPublicKey,
			RegistrationExpiry:    expiry,
			RemainingBalanceOwner: balanceOwnersAux,
			DisableOwner:          disableOwnersAux,
		},
		weight,
	)
}

func GetRegisterL1ValidatorMessage(
	ctx context.Context,
	rpcURL string,
	network models.Network,
	aggregatorLogger logging.Logger,
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
	alreadyInitialized bool,
	initiateTxHash string,
	registerSubnetValidatorUnsignedMessage *warp.UnsignedMessage,
) (*warp.Message, ids.ID, error) {
	var (
		validationID ids.ID
		err          error
	)
	if registerSubnetValidatorUnsignedMessage == nil {
		if alreadyInitialized {
			validationID, err = validator.GetValidationID(
				rpcURL,
				managerAddress,
				nodeID,
			)
			if err != nil {
				return nil, ids.Empty, err
			}
			if initiateTxHash != "" {
				registerSubnetValidatorUnsignedMessage, err = GetRegisterL1ValidatorMessageFromTx(
					rpcURL,
					validationID,
					initiateTxHash,
				)
				if err != nil {
					return nil, ids.Empty, err
				}
			} else {
				registerSubnetValidatorUnsignedMessage, err = SearchForRegisterL1ValidatorMessage(
					rpcURL,
					validationID,
				)
				if err != nil {
					return nil, ids.Empty, err
				}
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
	} else {
		payload := registerSubnetValidatorUnsignedMessage.Payload
		addressedCall, err := warpPayload.ParseAddressedCall(payload)
		if err != nil {
			return nil, ids.Empty, fmt.Errorf("unexpected format on given registration warp message: %w", err)
		}
		reg, err := warpMessage.ParseRegisterL1Validator(addressedCall.Payload)
		if err != nil {
			return nil, ids.Empty, fmt.Errorf("unexpected format on given registration warp message: %w", err)
		}
		validationID = reg.ValidationID()
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		ctx,
		network.SDKNetwork(),
		aggregatorLogger,
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
	return contract.GetSmartContractCallResult[*big.Int]("weightToValue", out)
}

func GetPChainL1ValidatorRegistrationMessage(
	ctx context.Context,
	network models.Network,
	rpcURL string,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
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
		network.SDKNetwork(),
		aggregatorLogger,
		subnetID,
		aggregatorQuorumPercentage,
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
	generateRawTxOnly bool,
	ownerAddress common.Address,
	privateKey string, // not need to be owner atm
	l1ValidatorRegistrationSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		generateRawTxOnly,
		ownerAddress,
		privateKey,
		managerAddress,
		l1ValidatorRegistrationSignedMessage,
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
	generateRawTxOnly bool,
	ownerAddressStr string,
	ownerPrivateKey string,
	nodeID ids.NodeID,
	blsPublicKey []byte,
	expiry uint64,
	balanceOwners warpMessage.PChainOwner,
	disableOwners warpMessage.PChainOwner,
	weight uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogger logging.Logger,
	isPos bool,
	delegationFee uint16,
	stakeDuration time.Duration,
	validatorManagerAddressStr string,
	useACP99 bool,
	initiateTxHash string,
) (*warp.Message, ids.ID, *types.Transaction, error) {
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, ids.Empty, nil, err
	}
	blockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, ids.Empty, nil, err
	}
	managerAddress := common.HexToAddress(validatorManagerAddressStr)
	ownerAddress := common.HexToAddress(ownerAddressStr)

	alreadyInitialized := initiateTxHash != ""
	if validationID, err := validator.GetValidationID(
		rpcURL,
		managerAddress,
		nodeID,
	); err != nil {
		return nil, ids.Empty, nil, err
	} else if validationID != ids.Empty {
		alreadyInitialized = true
	}

	var receipt *types.Receipt
	if !alreadyInitialized {
		var tx *types.Transaction
		if isPos {
			stakeAmount, err := validatormanager.PoSWeightToValue(
				rpcURL,
				managerAddress,
				weight,
			)
			if err != nil {
				return nil, ids.Empty, nil, fmt.Errorf("failure obtaining value from weight: %w", err)
			}
			ux.Logger.PrintLineSeparator()
			ux.Logger.PrintToUser("Initializing validator registration with PoS validator manager")
			ux.Logger.PrintToUser("Using RPC URL: %s", rpcURL)
			ux.Logger.PrintToUser("NodeID: %s staking %s tokens", nodeID.String(), stakeAmount)
			ux.Logger.PrintLineSeparator()
			tx, receipt, err = InitializeValidatorRegistrationPoSNative(
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
					return nil, ids.Empty, nil, evm.TransactionError(tx, err, "failure initializing validator registration")
				}
				ux.Logger.PrintToUser(logging.LightBlue.Wrap("The validator registration was already initialized. Proceeding to the next step"))
				alreadyInitialized = true
			}
			ux.Logger.PrintToUser(fmt.Sprintf("Validator staked amount: %d", stakeAmount))
		} else {
			managerAddress = common.HexToAddress(validatorManagerAddressStr)
			tx, receipt, err = InitializeValidatorRegistrationPoA(
				rpcURL,
				managerAddress,
				generateRawTxOnly,
				ownerAddress,
				ownerPrivateKey,
				nodeID,
				blsPublicKey,
				expiry,
				balanceOwners,
				disableOwners,
				weight,
				useACP99,
			)
			if err != nil {
				if !errors.Is(err, validatormanager.ErrNodeAlreadyRegistered) {
					return nil, ids.Empty, nil, evm.TransactionError(tx, err, "failure initializing validator registration")
				}
				ux.Logger.PrintToUser(logging.LightBlue.Wrap("The validator registration was already initialized. Proceeding to the next step"))
				alreadyInitialized = true
			} else if generateRawTxOnly {
				return nil, ids.Empty, tx, nil
			}
			ux.Logger.PrintToUser(fmt.Sprintf("Validator weight: %d", weight))
		}
	} else {
		ux.Logger.PrintToUser(logging.LightBlue.Wrap("The validator registration was already initialized. Proceeding to the next step"))
	}

	var unsignedMessage *warp.UnsignedMessage
	if receipt != nil {
		unsignedMessage, err = evm.ExtractWarpMessageFromReceipt(receipt)
		if err != nil {
			return nil, ids.Empty, nil, err
		}
	}

	signedMessage, validationID, err := GetRegisterL1ValidatorMessage(
		ctx,
		rpcURL,
		network,
		aggregatorLogger,
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
		alreadyInitialized,
		initiateTxHash,
		unsignedMessage,
	)

	return signedMessage, validationID, nil, err
}

func FinishValidatorRegistration(
	ctx context.Context,
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	generateRawTxOnly bool,
	ownerAddressStr string,
	privateKey string,
	validationID ids.ID,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogger logging.Logger,
	validatorManagerAddressStr string,
) (*types.Transaction, error) {
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, err
	}
	managerAddress := common.HexToAddress(validatorManagerAddressStr)
	signedMessage, err := GetPChainL1ValidatorRegistrationMessage(
		ctx,
		network,
		rpcURL,
		aggregatorLogger,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		validationID,
		true,
	)
	if err != nil {
		return nil, err
	}
	if privateKey != "" {
		if client, err := evm.GetClient(rpcURL); err != nil {
			ux.Logger.RedXToUser("failure connecting to L1 to setup proposer VM: %w", err)
		} else {
			if err := client.SetupProposerVM(privateKey); err != nil {
				ux.Logger.RedXToUser("failure setting proposer VM on L1: %w", err)
			}
			client.Close()
		}
	}
	ownerAddress := common.HexToAddress(ownerAddressStr)
	tx, _, err := CompleteValidatorRegistration(
		rpcURL,
		managerAddress,
		generateRawTxOnly,
		ownerAddress,
		privateKey,
		signedMessage,
	)
	if err != nil {
		if !errors.Is(err, validatormanager.ErrInvalidValidationID) {
			return nil, evm.TransactionError(tx, err, "failure completing validator registration")
		} else {
			return nil, fmt.Errorf("the Validator was already fully registered on the Manager")
		}
	}
	if generateRawTxOnly {
		return tx, nil
	}
	return nil, nil
}

func SearchForRegisterL1ValidatorMessage(
	rpcURL string,
	validationID ids.ID,
) (*warp.UnsignedMessage, error) {
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return nil, err
	}
	height, err := client.BlockNumber()
	if err != nil {
		return nil, err
	}
	maxBlock := int64(height)
	minBlock := int64(0)
	for blockNumber := maxBlock; blockNumber >= minBlock; blockNumber-- {
		block, err := client.BlockByNumber(big.NewInt(blockNumber))
		if err != nil {
			return nil, err
		}
		blockHash := block.Hash()
		logs, err := client.FilterLogs(interfaces.FilterQuery{
			BlockHash: &blockHash,
			Addresses: []common.Address{subnetEvmWarp.Module.Address},
		})
		if err != nil {
			return nil, err
		}
		msgs := evm.GetWarpMessagesFromLogs(utils.PointersSlice(logs))
		for _, msg := range msgs {
			payload := msg.Payload
			addressedCall, err := warpPayload.ParseAddressedCall(payload)
			if err == nil {
				reg, err := warpMessage.ParseRegisterL1Validator(addressedCall.Payload)
				if err == nil {
					if reg.ValidationID() == validationID {
						return msg, nil
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
	msg, err := SearchForRegisterL1ValidatorMessage(
		rpcURL,
		validationID,
	)
	if err != nil {
		return nil, err
	}
	payload := msg.Payload
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

func GetRegisterL1ValidatorMessageFromTx(
	rpcURL string,
	validationID ids.ID,
	txHash string,
) (*warp.UnsignedMessage, error) {
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return nil, err
	}
	receipt, err := client.TransactionReceipt(common.HexToHash(txHash))
	if err != nil {
		return nil, err
	}
	msgs := evm.GetWarpMessagesFromLogs(receipt.Logs)
	for _, msg := range msgs {
		payload := msg.Payload
		addressedCall, err := warpPayload.ParseAddressedCall(payload)
		if err == nil {
			reg, err := warpMessage.ParseRegisterL1Validator(addressedCall.Payload)
			if err == nil {
				if reg.ValidationID() == validationID {
					return msg, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("register validator message not found on tx %s", txHash)
}
