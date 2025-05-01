// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/sdk/evm"
	"github.com/ava-labs/avalanchego/ids"
	warpMessage "github.com/ava-labs/avalanchego/vms/platformvm/warp/message"
	warpPayload "github.com/ava-labs/avalanchego/vms/platformvm/warp/payload"
	"github.com/ava-labs/subnet-evm/interfaces"
	subnetEvmWarp "github.com/ava-labs/subnet-evm/precompile/contracts/warp"

	"github.com/ethereum/go-ethereum/common"
)

func GetValidatorNonce(
	rpcURL string,
	validationID ids.ID,
) (uint64, error) {
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return 0, err
	}
	height, err := client.BlockNumber()
	if err != nil {
		return 0, err
	}
	count := uint64(0)
	maxBlock := int64(height)
	minBlock := int64(0)
	for blockNumber := maxBlock; blockNumber >= minBlock; blockNumber-- {
		block, err := client.BlockByNumber(big.NewInt(blockNumber))
		if err != nil {
			return 0, err
		}
		blockHash := block.Hash()
		logs, err := client.FilterLogs(interfaces.FilterQuery{
			BlockHash: &blockHash,
			Addresses: []common.Address{subnetEvmWarp.Module.Address},
		})
		if err != nil {
			return 0, err
		}
		msgs := evm.GetWarpMessagesFromLogs(utils.PointersSlice(logs))
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
