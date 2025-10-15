// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	contractSDK "github.com/ava-labs/avalanche-tooling-sdk-go/evm/contract"
	"github.com/ava-labs/avalanche-tooling-sdk-go/interchain"
	"github.com/ava-labs/avalanche-tooling-sdk-go/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/core/types"
	"github.com/ava-labs/subnet-evm/warp/messages"
)

func InitializeValidatorRemoval(
	logger logging.Logger,
	rpcURL string,
	managerAddress common.Address,
	signer *evm.Signer,
	validationID ids.ID,
	isPoS bool,
	uptimeProofSignedMessage *warp.Message,
	force bool,
	useACP99 bool,
) (*types.Transaction, *types.Receipt, error) {
	if isPoS {
		if useACP99 {
			validatorInfo, err := validatormanager.GetValidator(rpcURL, managerAddress, validationID)
			if err != nil {
				return nil, nil, err
			}
			stakingValidatorInfo, err := validatormanager.GetStakingValidator(rpcURL, managerAddress, validationID)
			if err != nil {
				return nil, nil, err
			}
			if stakingValidatorInfo.MinStakeDuration != 0 {
				// proper PoS validator (it may be bootstrap PoS or non bootstrap PoA previous to a migration)
				if signer.Address() != stakingValidatorInfo.Owner {
					return nil, nil, fmt.Errorf("%s doesn't have authorization to remove the validator, should be %s", signer.Address(), stakingValidatorInfo.Owner)
				}
				startTime := time.Unix(int64(validatorInfo.StartTime), 0)
				endTime := startTime.Add(time.Second * time.Duration(stakingValidatorInfo.MinStakeDuration))
				endTimeStr := endTime.Format("2006-01-02 15:04:05")
				if !time.Now().After(endTime) {
					return nil, nil, fmt.Errorf("can't remove validator before %s", endTimeStr)
				}
			}
			if force {
				return contractSDK.TxToMethod(
					logger,
					rpcURL,
					signer,
					managerAddress,
					big.NewInt(0),
					"force POS validator removal",
					validatormanager.ErrorSignatureToError,
					"forceInitiateValidatorRemoval(bytes32,bool,uint32)",
					validationID,
					false, // no uptime proof if force
					uint32(0),
				)
			}
			// remove PoS validator with uptime proof
			return contractSDK.TxToMethodWithWarpMessage(
				logger,
				rpcURL,
				signer,
				managerAddress,
				uptimeProofSignedMessage,
				big.NewInt(0),
				"POS validator removal with uptime proof",
				validatormanager.ErrorSignatureToError,
				"initiateValidatorRemoval(bytes32,bool,uint32)",
				validationID,
				true, // submit uptime proof
				uint32(0),
			)
		}
		if force {
			return contractSDK.TxToMethod(
				logger,
				rpcURL,
				signer,
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
		return contractSDK.TxToMethodWithWarpMessage(
			logger,
			rpcURL,
			signer,
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
		return contractSDK.TxToMethod(
			logger,
			rpcURL,
			signer,
			managerAddress,
			big.NewInt(0),
			"POA validator removal initialization",
			validatormanager.ErrorSignatureToError,
			"initiateValidatorRemoval(bytes32)",
			validationID,
		)
	}
	return contractSDK.TxToMethod(
		logger,
		rpcURL,
		signer,
		managerAddress,
		big.NewInt(0),
		"POA validator removal initialization",
		validatormanager.ErrorSignatureToError,
		"initializeEndValidation(bytes32)",
		validationID,
	)
}

func GetUptimeProofMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	l1SubnetID ids.ID,
	l1BlockchainID ids.ID,
	validationID ids.ID,
	uptime uint64,
	signatureAggregatorEndpoint string,
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
		l1BlockchainID,
		addressedCall.Bytes(),
	)
	if err != nil {
		return nil, err
	}

	messageHexStr := hex.EncodeToString(uptimeProofUnsignedMessage.Bytes())
	return interchain.SignMessage(
		aggregatorLogger,
		signatureAggregatorEndpoint,
		messageHexStr,
		"",
		l1SubnetID.String(),
		aggregatorQuorumPercentage,
	)
}

func InitValidatorRemoval(
	ctx context.Context,
	logger logging.Logger,
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	l1RPCURL string,
	generateRawTxOnly bool,
	ownerSigner *evm.Signer,
	nodeID ids.NodeID,
	aggregatorLogger logging.Logger,
	isPoS bool,
	uptimeSec uint64,
	force bool,
	managerBlockchainID ids.ID,
	managerAddressStr string,
	useACP99 bool,
	initiateTxHash string,
	signatureAggregatorEndpoint string,
) (*warp.Message, ids.ID, *types.Transaction, error) {
	l1SubnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, ids.Empty, nil, err
	}

	l1BlockchainID, err := contract.GetBlockchainID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, ids.Empty, nil, err
	}

	managerSubnetID, err := contract.GetSubnetID(
		app,
		network,
		contract.ChainSpec{
			BlockchainID: managerBlockchainID.String(),
		},
	)
	if err != nil {
		return nil, ids.Empty, nil, err
	}

	managerAddress := common.HexToAddress(managerAddressStr)

	validationID, err := validatormanager.GetValidationID(
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
	if err != nil {
		return nil, ids.Empty, nil, err
	}

	var unsignedMessage *warp.UnsignedMessage
	if initiateTxHash != "" {
		unsignedMessage, err = GetL1ValidatorWeightMessageFromTx(
			rpcURL,
			validationID,
			0,
			initiateTxHash,
		)
		if err != nil {
			return nil, ids.Empty, nil, err
		}
	}

	var receipt *types.Receipt
	if unsignedMessage == nil {
		signedUptimeProof := &warp.Message{}
		if isPoS {
			if uptimeSec == 0 {
				uptimeSec, err = utils.GetL1ValidatorUptimeSeconds(l1RPCURL, nodeID)
				if err != nil {
					return nil, ids.Empty, nil, evm.TransactionError(nil, err, "failure getting uptime data for nodeID: %s via %s ", nodeID, rpcURL)
				}
			}
			logger.Info(fmt.Sprintf("Using uptime: %ds", uptimeSec))
			signedUptimeProof, err = GetUptimeProofMessage(
				network,
				aggregatorLogger,
				0,
				l1SubnetID,
				l1BlockchainID,
				validationID,
				uptimeSec,
				signatureAggregatorEndpoint,
			)
			if err != nil {
				return nil, ids.Empty, nil, evm.TransactionError(nil, err, "failure getting uptime proof")
			}
		}
		var tx *types.Transaction
		tx, receipt, err = InitializeValidatorRemoval(
			logger,
			rpcURL,
			managerAddress,
			ownerSigner,
			validationID,
			isPoS,
			signedUptimeProof, // is empty for non-PoS
			force,
			useACP99,
		)
		switch {
		case err != nil:
			if !errors.Is(err, validatormanager.ErrInvalidValidatorStatus) {
				return nil, ids.Empty, nil, evm.TransactionError(tx, err, "failure initializing validator removal")
			}
			logger.Info(logging.LightBlue.Wrap("The validator removal process was already initialized. Proceeding to the next step"))
		case generateRawTxOnly:
			return nil, ids.Empty, tx, nil
		default:
			logger.Info(fmt.Sprintf("Validator removal initialized. InitiateTxHash: %s", tx.Hash()))
		}
	} else {
		logger.Info(logging.LightBlue.Wrap("The validator removal process was already initialized. Proceeding to the next step"))
	}

	if receipt != nil {
		unsignedMessage, err = evm.ExtractWarpMessageFromReceipt(receipt)
		if err != nil {
			return nil, ids.Empty, nil, err
		}
	}

	var nonce uint64
	if unsignedMessage == nil {
		nonce, err = GetValidatorNonce(ctx, rpcURL, validationID)
		if err != nil {
			return nil, ids.Empty, nil, err
		}
	}

	signedMsg, err := GetL1ValidatorWeightMessage(
		network,
		aggregatorLogger,
		unsignedMessage,
		managerSubnetID,
		managerBlockchainID,
		managerAddress,
		validationID,
		nonce,
		0,
		signatureAggregatorEndpoint,
	)
	return signedMsg, validationID, nil, err
}

func CompleteValidatorRemoval(
	logger logging.Logger,
	rpcURL string,
	managerAddress common.Address,
	signer *evm.Signer, // not need to be owner atm
	subnetValidatorRegistrationSignedMessage *warp.Message,
	useACP99 bool,
) (*types.Transaction, *types.Receipt, error) {
	if useACP99 {
		return contractSDK.TxToMethodWithWarpMessage(
			logger,
			rpcURL,
			signer,
			managerAddress,
			subnetValidatorRegistrationSignedMessage,
			big.NewInt(0),
			"complete validator removal",
			validatormanager.ErrorSignatureToError,
			"completeValidatorRemoval(uint32)",
			uint32(0),
		)
	}
	return contractSDK.TxToMethodWithWarpMessage(
		logger,
		rpcURL,
		signer,
		managerAddress,
		subnetValidatorRegistrationSignedMessage,
		big.NewInt(0),
		"complete validator removal",
		validatormanager.ErrorSignatureToError,
		"completeEndValidation(uint32)",
		uint32(0),
	)
}

func FinishValidatorRemoval(
	ctx context.Context,
	logger logging.Logger,
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	chainSpec contract.ChainSpec,
	generateRawTxOnly bool,
	signer *evm.Signer,
	validationID ids.ID,
	aggregatorLogger logging.Logger,
	managerBlockchainID ids.ID,
	managerAddressStr string,
	useACP99 bool,
	signatureAggregatorEndpoint string,
) (*types.Transaction, error) {
	managerAddress := common.HexToAddress(managerAddressStr)
	subnetID, err := contract.GetSubnetID(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, err
	}

	managerSubnetID, err := contract.GetSubnetID(
		app,
		network,
		contract.ChainSpec{
			BlockchainID: managerBlockchainID.String(),
		},
	)
	if err != nil {
		return nil, err
	}

	signedMessage, err := GetPChainL1ValidatorRegistrationMessage(
		ctx,
		network,
		rpcURL,
		aggregatorLogger,
		0,
		subnetID,
		managerSubnetID,
		validationID,
		false,
		signatureAggregatorEndpoint,
	)
	if err != nil {
		return nil, err
	}
	if isNoOp, err := signer.IsNoOp(); err != nil {
		return nil, err
	} else if !isNoOp {
		if client, err := evm.GetClient(rpcURL); err != nil {
			logger.Error(fmt.Sprintf("failure connecting to L1 to setup proposer VM: %s", err))
		} else {
			if err := client.SetupProposerVM(signer); err != nil {
				logger.Error(fmt.Sprintf("failure setting proposer VM on L1: %s", err))
			}
			client.Close()
		}
	}
	tx, _, err := CompleteValidatorRemoval(
		logger,
		rpcURL,
		managerAddress,
		signer,
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
