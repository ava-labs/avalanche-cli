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
) (*types.Transaction, *types.Receipt, error) {
	if isPoS {
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
			// Ensure blockchain timestamp has also passed the min stake duration
			// eth_estimateGas uses current block timestamp, so we need to make sure
			// that timestamp is past the threshold, not just system time
			if err := ensureBlockchainTimestampPassed(logger, rpcURL, signer, validatorInfo.StartTime, stakingValidatorInfo.MinStakeDuration); err != nil {
				return nil, nil, err
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
	// PoA case
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

func GetUptimeProofMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	l1SubnetID ids.ID,
	l1BlockchainID ids.ID,
	validationID ids.ID,
	uptime uint64,
	signatureAggregatorEndpoint string,
	pChainHeight uint64,
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
		pChainHeight,
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
	initiateTxHash string,
	signatureAggregatorEndpoint string,
	pChainHeight uint64,
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
				pChainHeight,
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
		pChainHeight,
	)
	return signedMsg, validationID, nil, err
}

func CompleteValidatorRemoval(
	logger logging.Logger,
	rpcURL string,
	managerAddress common.Address,
	signer *evm.Signer, // not need to be owner atm
	subnetValidatorRegistrationSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
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

func FinishValidatorRemoval(
	ctx context.Context,
	logger logging.Logger,
	network models.Network,
	rpcURL string,
	subnetID ids.ID,
	managerSubnetID ids.ID,
	generateRawTxOnly bool,
	signer *evm.Signer,
	validationID ids.ID,
	managerAddressStr string,
	signedMessageParams SignatureAggregatorParams,
) (*types.Transaction, error) {
	managerAddress := common.HexToAddress(managerAddressStr)

	signedMessage, err := GetPChainL1ValidatorRegistrationMessage(
		ctx,
		network,
		rpcURL,
		subnetID,
		managerSubnetID,
		validationID,
		false,
		signedMessageParams,
	)
	if err != nil {
		return nil, err
	}
	tx, _, err := CompleteValidatorRemoval(
		logger,
		rpcURL,
		managerAddress,
		signer,
		signedMessage,
	)
	if err != nil {
		return nil, evm.TransactionError(tx, err, "failure completing validator removal")
	}
	if generateRawTxOnly {
		return tx, nil
	}
	return nil, nil
}

// ensureBlockchainTimestampPassed checks if the current blockchain timestamp
// has passed the validator start time + min stake duration. If not, it creates
// a dummy block to advance the timestamp. This is necessary because eth_estimateGas
// uses the current block timestamp, not the future block timestamp.
func ensureBlockchainTimestampPassed(
	logger logging.Logger,
	rpcURL string,
	signer *evm.Signer,
	validatorStartTime uint64,
	minStakeDuration uint64,
) error {
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to get client: %w", err)
	}
	defer client.Close()

	// Get the current block to check its timestamp
	currentBlock, err := client.BlockNumber()
	if err != nil {
		return fmt.Errorf("failed to get current block number: %w", err)
	}

	block, err := client.BlockByNumber(big.NewInt(int64(currentBlock)))
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}

	requiredTimestamp := validatorStartTime + minStakeDuration
	currentTimestamp := block.Time()

	if currentTimestamp >= requiredTimestamp {
		// Blockchain timestamp is already past the threshold
		return nil
	}

	// Need to create a dummy block to advance the timestamp
	secondsNeeded := requiredTimestamp - currentTimestamp
	logger.Info(fmt.Sprintf("Current block timestamp (%d) is %d seconds before required timestamp (%d)",
		currentTimestamp, secondsNeeded, requiredTimestamp))
	logger.Info("Creating a dummy block to advance blockchain timestamp...")

	// Send a minimal self-transfer to trigger block creation
	receipt, err := client.FundAddress(signer, signer.Address().Hex(), big.NewInt(0))
	if err != nil {
		return fmt.Errorf("failed to create dummy block: %w", err)
	}

	logger.Info(fmt.Sprintf("Dummy block created with transaction %s", receipt.TxHash))

	// Verify the new block timestamp
	newBlock, err := client.BlockByNumber(nil) // nil means latest block
	if err != nil {
		return fmt.Errorf("failed to get new block: %w", err)
	}

	newTimestamp := newBlock.Time()
	if newTimestamp < requiredTimestamp {
		return fmt.Errorf("new block timestamp (%d) is still before required timestamp (%d)", newTimestamp, requiredTimestamp)
	}

	logger.Info(fmt.Sprintf("New block timestamp (%d) is now past required timestamp (%d)", newTimestamp, requiredTimestamp))
	return nil
}
