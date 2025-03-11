// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"
	warp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ava-labs/subnet-evm/interfaces"
	subnetEvmWarp "github.com/ava-labs/subnet-evm/precompile/contracts/warp"

	"github.com/ethereum/go-ethereum/common"
)

func GetWarpMessagesFromLogs(
	logs []*types.Log,
) []*warp.UnsignedMessage {
	messages := []*warp.UnsignedMessage{}
	for _, txLog := range logs {
		msg, err := subnetEvmWarp.UnpackSendWarpEventDataToMessage(txLog.Data)
		if err == nil {
			messages = append(messages, msg)
		}
	}
	return messages
}

func GetWarpMessageFromLogs(
	logs []*types.Log,
) (*warp.UnsignedMessage, error) {
	messages := GetWarpMessagesFromLogs(logs)
	if len(messages) == 0 {
		return nil, fmt.Errorf("no warp message is present in evm logs")
	}
	return messages[0], nil
}

func GetValidatorNonce(
	rpcURL string,
	validationID ids.ID,
) (uint64, error) {
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return 0, err
	}
	ctx, cancel := utils.GetAPILargeContext()
	defer cancel()
	height, err := client.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}
	count := uint64(0)
	maxBlock := int64(height)
	minBlock := int64(0)
	for blockNumber := maxBlock; blockNumber >= minBlock; blockNumber-- {
		ctx, cancel := utils.GetAPILargeContext()
		defer cancel()
		block, err := client.BlockByNumber(ctx, big.NewInt(blockNumber))
		if err != nil {
			return 0, err
		}
		blockHash := block.Hash()
		logs, err := client.FilterLogs(ctx, interfaces.FilterQuery{
			BlockHash: &blockHash,
			Addresses: []common.Address{subnetEvmWarp.Module.Address},
		})
		if err != nil {
			return 0, err
		}
		msgs := GetWarpMessagesFromLogs(utils.PointersSlice(logs))
		for _, msg := range msgs {
			payload := msg.Payload
			addressedCall, err := warpPayload.ParseAddressedCall(payload)
			if err == nil {
				weightMsg, err := warpMessage.ParseL1ValidatorWeight(addressedCall.Payload)
				if err == nil {
					if weightMsg.ValidationID == validationID {
						count++
					}
				}
				regMsg, err := warpMessage.ParseRegisterL1Validator(addressedCall.Payload)
				if err == nil {
					if regMsg.ValidationID() == validationID {
						return count, nil
					}
				}
			}
		}
	}
	return count, nil
}
