// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	"context"
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
	"github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/warp/messages"

	"github.com/ethereum/go-ethereum/common"
)

func InitializeValidatorRemoval(
	rpcURL string,
	managerAddress common.Address,
	generateRawTxOnly bool,
	managerOwnerAddress common.Address,
	privateKey string,
	validationID ids.ID,
	isPoS bool,
	uptimeProofSignedMessage *warp.Message,
	force bool,
	useACP99 bool,
) (*types.Transaction, *types.Receipt, error) {
	if isPoS {
		if force {
			return contract.TxToMethod(
				rpcURL,
				false,
				common.Address{},
				privateKey,
				managerAddress,
				big.NewInt(0),
				"force POS validator removal",
				validatormanager.ErrorSignatureToError,
				"forceInitializeEndValidation(bytes32,bool,uint32)",
				validationID,
				false, // no uptime proof if force
				uint32(0),
			)
		}
		// remove PoS validator with uptime proof
		return contract.TxToMethodWithWarpMessage(
			rpcURL,
			false,
			common.Address{},
			privateKey,
			managerAddress,
			uptimeProofSignedMessage,
			big.NewInt(0),
			"POS validator removal with uptime proof",
			validatormanager.ErrorSignatureToError,
			"initializeEndValidation(bytes32,bool,uint32)",
			validationID,
			true, // submit uptime proof
			uint32(0),
		)
	}
	// PoA case
	if useACP99 {
		return contract.TxToMethod(
			rpcURL,
			generateRawTxOnly,
			managerOwnerAddress,
			privateKey,
			managerAddress,
			big.NewInt(0),
			"POA validator removal initialization",
			validatormanager.ErrorSignatureToError,
			"initiateValidatorRemoval(bytes32)",
			validationID,
		)
	}
	return contract.TxToMethod(
		rpcURL,
		generateRawTxOnly,
		managerOwnerAddress,
		privateKey,
		managerAddress,
		big.NewInt(0),
		"POA validator removal initialization",
		validatormanager.ErrorSignatureToError,
		"initializeEndValidation(bytes32)",
		validationID,
	)
}

func GetUptimeProofMessage(
	ctx context.Context,
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	blockchainID ids.ID,
	validationID ids.ID,
	uptime uint64,
) (*warp.Message, error) {
	uptimePayload, err := messages.NewValidatorUptime(validationID, uptime)
	if err != nil {
		return nil, err
	}
	addressedCall, err := warpPayload.NewAddressedCall(nil, uptimePayload.Bytes())
	if err != nil {
		return nil, err
	}
	uptimeProofUnsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		blockchainID,
		addressedCall.Bytes(),
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
		true, // allow private peers
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(uptimeProofUnsignedMessage, nil)
}

func InitValidatorRemoval(
	ctx context.Context,
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	generateRawTxOnly bool,
	ownerAddressStr string,
	ownerPrivateKey string,
	nodeID ids.NodeID,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	isPoS bool,
	uptimeSec uint64,
	force bool,
	validatorManagerAddressStr string,
	useACP99 bool,
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
	validationID, err := validator.GetRegisteredValidator(
		rpcURL,
		managerAddress,
		nodeID,
	)
	if err != nil {
		return nil, ids.Empty, nil, err
	}
	if validationID == ids.Empty {
		return nil, ids.Empty, nil, fmt.Errorf("node %s is not a L1 validator", nodeID)
	}

	signedUptimeProof := &warp.Message{}
	if isPoS {
		if uptimeSec == 0 {
			uptimeSec, err = utils.GetL1ValidatorUptimeSeconds(rpcURL, nodeID)
			if err != nil {
				return nil, ids.Empty, nil, evm.TransactionError(nil, err, "failure getting uptime data for nodeID: %s via %s ", nodeID, rpcURL)
			}
		}
		ux.Logger.PrintToUser("Using uptime: %ds", uptimeSec)
		signedUptimeProof, err = GetUptimeProofMessage(
			ctx,
			network,
			aggregatorLogger,
			0,
			aggregatorExtraPeerEndpoints,
			subnetID,
			blockchainID,
			validationID,
			uptimeSec,
		)
		if err != nil {
			return nil, ids.Empty, nil, evm.TransactionError(nil, err, "failure getting uptime proof")
		}
	}
	tx, receipt, err := InitializeValidatorRemoval(
		rpcURL,
		managerAddress,
		generateRawTxOnly,
		ownerAddress,
		ownerPrivateKey,
		validationID,
		isPoS,
		signedUptimeProof, // is empty for non-PoS
		force,
		useACP99,
	)
	if err != nil {
		if !errors.Is(err, validatormanager.ErrInvalidValidatorStatus) {
			return nil, ids.Empty, nil, evm.TransactionError(tx, err, "failure initializing validator removal")
		}
		ux.Logger.PrintToUser(logging.LightBlue.Wrap("The validator removal process was already initialized. Proceeding to the next step"))
	} else if generateRawTxOnly {
		return nil, ids.Empty, tx, nil
	}

	var unsignedMessage *warp.UnsignedMessage
	if receipt != nil {
		unsignedMessage, err = GetWarpMessageFromLogs(receipt.Logs)
		if err != nil {
			return nil, ids.Empty, nil, err
		}
	}

	var nonce uint64
	if unsignedMessage == nil {
		nonce, err = GetValidatorNonce(rpcURL, validationID)
		if err != nil {
			return nil, ids.Empty, nil, err
		}
	}

	signedMsg, err := GetL1ValidatorWeightMessage(
		ctx,
		network,
		aggregatorLogger,
		0,
		aggregatorAllowPrivatePeers,
		aggregatorExtraPeerEndpoints,
		unsignedMessage,
		subnetID,
		blockchainID,
		managerAddress,
		validationID,
		nonce,
		0,
	)
	return signedMsg, validationID, nil, err
}

func CompleteValidatorRemoval(
	rpcURL string,
	managerAddress common.Address,
	generateRawTxOnly bool,
	ownerAddress common.Address,
	privateKey string, // not need to be owner atm
	subnetValidatorRegistrationSignedMessage *warp.Message,
	useACP99 bool,
) (*types.Transaction, *types.Receipt, error) {
	if useACP99 {
		return contract.TxToMethodWithWarpMessage(
			rpcURL,
			generateRawTxOnly,
			ownerAddress,
			privateKey,
			managerAddress,
			subnetValidatorRegistrationSignedMessage,
			big.NewInt(0),
			"complete poa validator removal",
			validatormanager.ErrorSignatureToError,
			"completeValidatorRemoval(uint32)",
			uint32(0),
		)
	}
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		generateRawTxOnly,
		ownerAddress,
		privateKey,
		managerAddress,
		subnetValidatorRegistrationSignedMessage,
		big.NewInt(0),
		"complete poa validator removal",
		validatormanager.ErrorSignatureToError,
		"completeEndValidation(uint32)",
		uint32(0),
	)
}

func FinishValidatorRemoval(
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
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	validatorManagerAddressStr string,
	useACP99 bool,
) (*types.Transaction, error) {
	managerAddress := common.HexToAddress(validatorManagerAddressStr)
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, err
	}
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
		false,
	)
	if err != nil {
		return nil, err
	}
	if privateKey != "" {
		if err := evm.SetupProposerVM(
			rpcURL,
			privateKey,
		); err != nil {
			ux.Logger.RedXToUser("failure setting proposer VM on L1: %w", err)
		}
	}
	ownerAddress := common.HexToAddress(ownerAddressStr)
	tx, _, err := CompleteValidatorRemoval(
		rpcURL,
		managerAddress,
		generateRawTxOnly,
		ownerAddress,
		privateKey,
		signedMessage,
		useACP99,
	)
	if err != nil {
		return nil, evm.TransactionError(tx, err, "failure completing validator removal")
	}
	if generateRawTxOnly {
		return tx, nil
	}
	return nil, nil
}
