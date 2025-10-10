// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-tooling-sdk-go/evm"
	contractSDK "github.com/ava-labs/avalanche-tooling-sdk-go/evm/contract"
	"github.com/ava-labs/avalanche-tooling-sdk-go/interchain"
	"github.com/ava-labs/avalanche-tooling-sdk-go/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	ethereum "github.com/ava-labs/libevm"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/libevm/core/types"
	subnetEvmWarp "github.com/ava-labs/subnet-evm/precompile/contracts/warp"
)

func InitializeValidatorWeightChange(
	logger logging.Logger,
	rpcURL string,
	managerAddress common.Address,
	signer *evm.Signer,
	validationID ids.ID,
	weight uint64,
) (*types.Transaction, *types.Receipt, error) {
	return contractSDK.TxToMethod(
		logger,
		rpcURL,
		signer,
		managerAddress,
		big.NewInt(0),
		"POA validator weight change initialization",
		validatormanager.ErrorSignatureToError,
		"initiateValidatorWeightUpdate(bytes32,uint64)",
		validationID,
		weight,
	)
}

func InitValidatorWeightChange(
	ctx context.Context,
	logger logging.Logger,
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	generateRawTxOnly bool,
	ownerSigner *evm.Signer,
	nodeID ids.NodeID,
	aggregatorLogger logging.Logger,
	managerBlockchainID ids.ID,
	managerAddressStr string,
	weight uint64,
	initiateTxHash string,
	signatureAggregatorEndpoint string,
) (*warp.Message, ids.ID, *types.Transaction, error) {
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

	var unsignedMessage *warp.UnsignedMessage
	if initiateTxHash != "" {
		unsignedMessage, err = GetL1ValidatorWeightMessageFromTx(
			rpcURL,
			validationID,
			weight,
			initiateTxHash,
		)
		if err != nil {
			return nil, ids.Empty, nil, err
		}
	}

	if unsignedMessage == nil {
		unsignedMessage, err = SearchForL1ValidatorWeightMessage(ctx, rpcURL, validationID, weight)
		if err != nil {
			logger.Error(fmt.Sprintf(logging.Red.Wrap("Failure checking for warp messages of previous operations: %s. Proceeding."), err))
		}
	}

	var receipt *types.Receipt
	if unsignedMessage == nil {
		var tx *types.Transaction
		tx, receipt, err = InitializeValidatorWeightChange(
			logger,
			rpcURL,
			managerAddress,
			ownerSigner,
			validationID,
			weight,
		)
		switch {
		case err != nil:
			return nil, ids.Empty, nil, evm.TransactionError(tx, err, "failure initializing validator weight change")
		case generateRawTxOnly:
			return nil, ids.Empty, tx, nil
		default:
			ux.Logger.PrintToUser("Validator weight change initialized. InitiateTxHash: %s", tx.Hash())
		}
	} else {
		logger.Info(logging.LightBlue.Wrap("The validator weight change process was already initialized. Proceeding to the next step"))
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
		weight,
		signatureAggregatorEndpoint,
	)
	return signedMsg, validationID, nil, err
}

func CompleteValidatorWeightChange(
	logger logging.Logger,
	rpcURL string,
	managerAddress common.Address,
	signer *evm.Signer, // not need to be owner atm
	pchainL1ValidatorRegistrationSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
	return contractSDK.TxToMethodWithWarpMessage(
		logger,
		rpcURL,
		signer,
		managerAddress,
		pchainL1ValidatorRegistrationSignedMessage,
		big.NewInt(0),
		"complete poa validator weight change",
		validatormanager.ErrorSignatureToError,
		"completeValidatorWeightUpdate(uint32)",
		uint32(0),
	)
}

func FinishValidatorWeightChange(
	ctx context.Context,
	logger logging.Logger,
	app *application.Avalanche,
	network models.Network,
	rpcURL string,
	generateRawTxOnly bool,
	signer *evm.Signer,
	validationID ids.ID,
	aggregatorLogger logging.Logger,
	managerBlockchainID ids.ID,
	managerAddressStr string,
	l1ValidatorRegistrationSignedMessage *warp.Message,
	weight uint64,
	signatureAggregatorEndpoint string,
) (*types.Transaction, error) {
	managerAddress := common.HexToAddress(managerAddressStr)

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

	var nonce uint64
	if l1ValidatorRegistrationSignedMessage == nil {
		nonce, err = GetValidatorNonce(ctx, rpcURL, validationID)
		if err != nil {
			return nil, err
		}
	}
	signedMessage, err := GetPChainL1ValidatorWeightMessage(
		network,
		aggregatorLogger,
		0,
		managerSubnetID,
		l1ValidatorRegistrationSignedMessage,
		validationID,
		nonce,
		weight,
		signatureAggregatorEndpoint,
	)
	if err != nil {
		return nil, err
	}
	if !signer.IsNoOp() {
		if client, err := evm.GetClient(rpcURL); err != nil {
			logger.Error(fmt.Sprintf("failure connecting to L1 to setup proposer VM: %s", err))
		} else {
			if err := client.SetupProposerVM(signer); err != nil {
				logger.Error(fmt.Sprintf("failure setting proposer VM on L1: %s", err))
			}
			client.Close()
		}
	}
	tx, _, err := CompleteValidatorWeightChange(
		logger,
		rpcURL,
		managerAddress,
		signer,
		signedMessage,
	)
	if err != nil {
		return nil, evm.TransactionError(tx, err, "failure completing validator weight change")
	}
	if generateRawTxOnly {
		return tx, nil
	}
	return nil, nil
}

func GetL1ValidatorWeightMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	// message is given
	unsignedMessage *warp.UnsignedMessage,
	managerSubnetID ids.ID,
	managerBlockchainID ids.ID,
	managerAddress common.Address,
	validationID ids.ID,
	nonce uint64,
	weight uint64,
	signatureAggregatorEndpoint string,
) (*warp.Message, error) {
	if unsignedMessage == nil {
		addressedCallPayload, err := warpMessage.NewL1ValidatorWeight(
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
		unsignedMessage, err = warp.NewUnsignedMessage(
			network.ID,
			managerBlockchainID,
			addressedCall.Bytes(),
		)
		if err != nil {
			return nil, err
		}
	}
	messageHexStr := hex.EncodeToString(unsignedMessage.Bytes())
	return interchain.SignMessage(
		aggregatorLogger,
		signatureAggregatorEndpoint,
		messageHexStr,
		"",
		managerSubnetID.String(),
		0,
	)
}

func GetPChainL1ValidatorWeightMessage(
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	managerSubnetID ids.ID,
	// message is given
	l1SignedMessage *warp.Message,
	// needed to generate full message contents
	validationID ids.ID,
	nonce uint64,
	weight uint64,
	signatureAggregatorEndpoint string,
) (*warp.Message, error) {
	if l1SignedMessage != nil {
		addressedCall, err := warpPayload.ParseAddressedCall(l1SignedMessage.UnsignedMessage.Payload)
		if err != nil {
			return nil, err
		}
		weightMsg, err := warpMessage.ParseL1ValidatorWeight(addressedCall.Payload)
		if err != nil {
			return nil, err
		}
		validationID = weightMsg.ValidationID
		nonce = weightMsg.Nonce
		weight = weightMsg.Weight
	}
	addressedCallPayload, err := warpMessage.NewL1ValidatorWeight(
		validationID,
		nonce,
		weight,
	)
	if err != nil {
		return nil, err
	}
	addressedCall, err := warpPayload.NewAddressedCall(
		nil,
		addressedCallPayload.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	unsignedMessage, err := warp.NewUnsignedMessage(
		network.ID,
		avagoconstants.PlatformChainID,
		addressedCall.Bytes(),
	)
	if err != nil {
		return nil, err
	}
	messageHexStr := hex.EncodeToString(unsignedMessage.Bytes())
	return interchain.SignMessage(
		aggregatorLogger,
		signatureAggregatorEndpoint,
		messageHexStr,
		"",
		managerSubnetID.String(),
		aggregatorQuorumPercentage,
	)
}

func GetL1ValidatorWeightMessageFromTx(
	rpcURL string,
	validationID ids.ID,
	weight uint64,
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
			weightMsg, err := warpMessage.ParseL1ValidatorWeight(addressedCall.Payload)
			if err == nil {
				if weightMsg.ValidationID == validationID && weightMsg.Weight == weight {
					return msg, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("weight message not found on tx %s", txHash)
}

func SearchForL1ValidatorWeightMessage(
	ctx context.Context,
	rpcURL string,
	validationID ids.ID,
	weight uint64,
) (*warp.UnsignedMessage, error) {
	maxBlocksToSearch := int64(5000000)
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return nil, err
	}
	height, err := client.BlockNumber()
	if err != nil {
		return nil, err
	}
	maxBlock := int64(height)
	minBlock := max(maxBlock-maxBlocksToSearch, 0)
	blockStep := int64(5000)
	for blockNumber := maxBlock; blockNumber >= minBlock; blockNumber -= blockStep {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		fromBlock := big.NewInt(blockNumber - blockStep)
		if fromBlock.Sign() < 0 {
			fromBlock = big.NewInt(0)
		}
		toBlock := big.NewInt(blockNumber)
		logs, err := client.FilterLogs(ethereum.FilterQuery{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
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
				weightMsg, err := warpMessage.ParseL1ValidatorWeight(addressedCall.Payload)
				if err == nil {
					if weightMsg.ValidationID == validationID && weightMsg.Weight == weight {
						return msg, nil
					} else {
						return nil, nil
					}
				}
			}
		}
	}
	return nil, nil
}
