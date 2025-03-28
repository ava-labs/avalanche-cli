// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	"context"
	_ "embed"
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
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/interfaces"
	subnetEvmWarp "github.com/ava-labs/subnet-evm/precompile/contracts/warp"

	"github.com/ethereum/go-ethereum/common"
)

func InitializeValidatorWeightChange(
	rpcURL string,
	managerAddress common.Address,
	generateRawTxOnly bool,
	managerOwnerAddress common.Address,
	privateKey string,
	validationID ids.ID,
	weight uint64,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethod(
		rpcURL,
		generateRawTxOnly,
		managerOwnerAddress,
		privateKey,
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
	printFunc func(msg string, args ...interface{}),
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
	validatorManagerAddressStr string,
	weight uint64,
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
	validationID, err := validator.GetValidationID(
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
		unsignedMessage, err = SearchForL1ValidatorWeightMessage(rpcURL, validationID, weight)
		if err != nil {
			printFunc(logging.Red.Wrap("Failure checking for warp messages of previous operations: %s. Proceeding."), err)
		}
	}

	var receipt *types.Receipt
	if unsignedMessage == nil {
		var tx *types.Transaction
		tx, receipt, err = InitializeValidatorWeightChange(
			rpcURL,
			managerAddress,
			generateRawTxOnly,
			ownerAddress,
			ownerPrivateKey,
			validationID,
			weight,
		)
		if err != nil {
			return nil, ids.Empty, nil, evm.TransactionError(tx, err, "failure initializing validator weight change")
		} else if generateRawTxOnly {
			return nil, ids.Empty, tx, nil
		}
	} else {
		printFunc(logging.LightBlue.Wrap("The validator weight change process was already initialized. Proceeding to the next step"))
	}

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
		weight,
	)
	return signedMsg, validationID, nil, err
}

func CompleteValidatorWeightChange(
	rpcURL string,
	managerAddress common.Address,
	generateRawTxOnly bool,
	ownerAddress common.Address,
	privateKey string, // not need to be owner atm
	pchainL1ValidatorRegistrationSignedMessage *warp.Message,
) (*types.Transaction, *types.Receipt, error) {
	return contract.TxToMethodWithWarpMessage(
		rpcURL,
		generateRawTxOnly,
		ownerAddress,
		privateKey,
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
	l1ValidatorRegistrationSignedMessage *warp.Message,
	weight uint64,
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
	var nonce uint64
	if l1ValidatorRegistrationSignedMessage == nil {
		nonce, err = GetValidatorNonce(rpcURL, validationID)
		if err != nil {
			return nil, err
		}
	}
	signedMessage, err := GetPChainL1ValidatorWeightMessage(
		ctx,
		network,
		aggregatorLogger,
		0,
		aggregatorAllowPrivatePeers,
		aggregatorExtraPeerEndpoints,
		subnetID,
		l1ValidatorRegistrationSignedMessage,
		validationID,
		nonce,
		weight,
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
	tx, _, err := CompleteValidatorWeightChange(
		rpcURL,
		managerAddress,
		generateRawTxOnly,
		ownerAddress,
		privateKey,
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
	ctx context.Context,
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorAllowPrivateIPs bool,
	aggregatorExtraPeerEndpoints []info.Peer,
	// message is given
	unsignedMessage *warp.UnsignedMessage,
	// needed to generate message
	subnetID ids.ID,
	blockchainID ids.ID,
	managerAddress common.Address,
	validationID ids.ID,
	nonce uint64,
	weight uint64,
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
			blockchainID,
			addressedCall.Bytes(),
		)
		if err != nil {
			return nil, err
		}
	}
	signatureAggregator, err := interchain.NewSignatureAggregator(
		ctx,
		network.SDKNetwork(),
		aggregatorLogger,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorAllowPrivateIPs,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(unsignedMessage, nil)
}

func GetPChainL1ValidatorWeightMessage(
	ctx context.Context,
	network models.Network,
	aggregatorLogger logging.Logger,
	aggregatorQuorumPercentage uint64,
	aggregatorAllowPrivateIPs bool,
	aggregatorExtraPeerEndpoints []info.Peer,
	subnetID ids.ID,
	// message is given
	l1SignedMessage *warp.Message,
	// needed to generate full message contents
	validationID ids.ID,
	nonce uint64,
	weight uint64,
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
	signatureAggregator, err := interchain.NewSignatureAggregator(
		ctx,
		network.SDKNetwork(),
		aggregatorLogger,
		subnetID,
		aggregatorQuorumPercentage,
		aggregatorAllowPrivateIPs,
		aggregatorExtraPeerEndpoints,
	)
	if err != nil {
		return nil, err
	}
	return signatureAggregator.Sign(unsignedMessage, nil)
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
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	receipt, err := client.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		return nil, err
	}
	msgs := GetWarpMessagesFromLogs(receipt.Logs)
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
	rpcURL string,
	validationID ids.ID,
	weight uint64,
) (*warp.UnsignedMessage, error) {
	const maxBlocksToSearch = 500
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
	maxBlock := int64(height)
	minBlock := max(maxBlock-maxBlocksToSearch, 0)
	for blockNumber := maxBlock; blockNumber >= minBlock; blockNumber-- {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		block, err := client.BlockByNumber(ctx, big.NewInt(blockNumber))
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
		msgs := GetWarpMessagesFromLogs(utils.PointersSlice(logs))
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
