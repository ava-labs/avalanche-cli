// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"errors"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/interchain"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
)

func ValidatorManagerInitializeValidatorRemoval(
	rpcURL string,
	managerAddress common.Address,
	ownerPrivateKey string,
	validationID [32]byte,
	pos bool,
) (*types.Transaction, *types.Receipt, error) {
	if pos {
		return contract.TxToMethod(
			rpcURL,
			ownerPrivateKey,
			managerAddress,
			big.NewInt(0),
			"validator removal initialization",
			validatorManagerSDK.ErrorSignatureToError,
			"initializeEndValidation(bytes32,bool,uint32)",
			validationID,
			false,
			uint32(0),
		)
	}
	return contract.TxToMethod(
		rpcURL,
		ownerPrivateKey,
		managerAddress,
		big.NewInt(0),
		"validator removal initialization",
		validatorManagerSDK.ErrorSignatureToError,
		"initializeEndValidation(bytes32)",
		validationID,
	)
}

func ValidatorManagerGetSubnetValidatorWeightMessage(
	network models.Network,
	aggregatorLogLevel logging.Level,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	blockchainID ids.ID,
	managerAddress common.Address,
	validationID ids.ID,
	nonce uint64,
	weight uint64,
) (*warp.Message, error) {
	addressedCallPayload, err := warpMessage.NewSubnetValidatorWeight(
		validationID,
		nonce,
		weight,
	)
	if err != nil {
		return nil, err
	}
	addressedCall, err := warpPayload.NewAddressedCall(
		managerAddress.Bytes(),
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	unsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		blockchainID,
		addressedCall.Bytes(),
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
	return signatureAggregator.Sign(unsignedMessage, nil)
}

func InitValidatorRemoval(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	ownerPrivateKey string,
	nodeID ids.NodeID,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogLevelStr string,
	initWithPos bool,
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
	managerAddress := common.HexToAddress(validatorManagerSDK.ProxyContractAddress)
	validationID, err := GetRegisteredValidator(
		rpcURL,
		managerAddress,
		nodeID,
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	ux.Logger.PrintToUser("DEBUGGING: Start initializing validator removal process... %s", validationID)
	tx, _, err := ValidatorManagerInitializeValidatorRemoval(
		rpcURL,
		managerAddress,
		ownerPrivateKey,
		validationID,
		initWithPos,
	)
	ux.Logger.PrintToUser("DEBUGGING: %v", err)
	if err != nil {
		if !errors.Is(err, validatorManagerSDK.ErrInvalidValidatorStatus) {
			return nil, ids.Empty, evm.TransactionError(tx, err, "failure initializing validator removal")
		}
		ux.Logger.PrintToUser("the validator removal process was already initialized. Proceeding to the next step")
	}

	aggregatorLogLevel, err := logging.ToLevel(aggregatorLogLevelStr)
	if err != nil {
		aggregatorLogLevel = defaultAggregatorLogLevel
	}
	nonce := uint64(1)
	signedMsg, err := ValidatorManagerGetSubnetValidatorWeightMessage(
		network,
		aggregatorLogLevel,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		blockchainID,
		managerAddress,
		validationID,
		nonce,
		0,
	)
	return signedMsg, validationID, err
}

func ValidatorManagerCompleteValidatorRemoval(
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
		"complete poa validator removal",
		validatorManagerSDK.ErrorSignatureToError,
		"completeEndValidation(uint32)",
		uint32(0),
	)
}

func FinishValidatorRemoval(
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	privateKey string,
	validationID ids.ID,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorLogLevelStr string,
) error {
	managerAddress := common.HexToAddress(validatorManagerSDK.ProxyContractAddress)
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
	signedMessage, err := ValidatorManagerGetPChainSubnetValidatorRegistrationWarpMessage(
		network,
		rpcURL,
		aggregatorLogLevel,
		0,
		aggregatorExtraPeerEndpoints,
		subnetID,
		validationID,
		false,
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
	tx, _, err := ValidatorManagerCompleteValidatorRemoval(
		rpcURL,
		managerAddress,
		privateKey,
		signedMessage,
	)
	if err != nil {
		return evm.TransactionError(tx, err, "failure completing validator removal")
	}
	return nil
}
