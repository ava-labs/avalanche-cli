// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/subnet-evm/core/types"
	subnetevmwarp "github.com/ava-labs/subnet-evm/precompile/contracts/warp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestGetWarpMessagesFromLogs(t *testing.T) {
	// Test case 1: Empty logs
	logs := []*types.Log{}
	messages := GetWarpMessagesFromLogs(logs)
	require.Empty(t, messages)
	// Test case 2: Log with invalid warp message
	invalidPayload := []byte{1, 2, 3, 4, 5}
	invalidLog := &types.Log{
		Data: invalidPayload,
	}
	logs = []*types.Log{invalidLog}
	messages = GetWarpMessagesFromLogs(logs)
	require.Empty(t, messages)
	// Test case 3: Log with valid warp message
	unsignedWarpMessage, err := warp.NewUnsignedMessage(
		0,
		ids.ID{},
		[]byte{},
	)
	require.NoError(t, err)
	_, validPayload, err := subnetevmwarp.PackSendWarpMessageEvent(
		common.Address{},
		common.Hash{},
		unsignedWarpMessage.Bytes(),
	)
	require.NoError(t, err)
	validLog := &types.Log{
		Data: validPayload,
	}
	logs = []*types.Log{validLog}
	messages = GetWarpMessagesFromLogs(logs)
	require.Equal(t, messages, []*warp.UnsignedMessage{unsignedWarpMessage})
	// Test case 4: Multiple logs with mixed valid and invalid messages
	logs = []*types.Log{validLog, invalidLog}
	messages = GetWarpMessagesFromLogs(logs)
	require.Equal(t, messages, []*warp.UnsignedMessage{unsignedWarpMessage})
}

func TestExtractWarpMessageFromLogs(t *testing.T) {
	// Test case 1: Empty logs
	logs := []*types.Log{}
	msg, err := ExtractWarpMessageFromLogs(logs)
	require.Error(t, err)
	require.Nil(t, msg)
	require.Contains(t, err.Error(), "no warp message is present in evm logs")
	// Test case 2: Log with invalid warp message
	invalidPayload := []byte{1, 2, 3, 4, 5}
	invalidLog := &types.Log{
		Data: invalidPayload,
	}
	logs = []*types.Log{invalidLog}
	msg, err = ExtractWarpMessageFromLogs(logs)
	require.Error(t, err)
	require.Nil(t, msg)
	require.Contains(t, err.Error(), "no warp message is present in evm logs")
	// Test case 3: Log with valid warp message
	unsignedWarpMessage, err := warp.NewUnsignedMessage(
		0,
		ids.ID{},
		[]byte{},
	)
	require.NoError(t, err)
	_, validPayload, err := subnetevmwarp.PackSendWarpMessageEvent(
		common.Address{},
		common.Hash{},
		unsignedWarpMessage.Bytes(),
	)
	require.NoError(t, err)
	validLog := &types.Log{
		Data: validPayload,
	}
	logs = []*types.Log{validLog}
	msg, err = ExtractWarpMessageFromLogs(logs)
	require.NoError(t, err)
	require.Equal(t, unsignedWarpMessage, msg)
	// Test case 4: Multiple logs with mixed valid and invalid messages
	logs = []*types.Log{validLog, invalidLog}
	msg, err = ExtractWarpMessageFromLogs(logs)
	require.NoError(t, err)
	require.Equal(t, unsignedWarpMessage, msg)
}

func TestExtractWarpMessageFromReceipt(t *testing.T) {
	// Test case 1: Nil receipt
	msg, err := ExtractWarpMessageFromReceipt(nil)
	require.Error(t, err)
	require.Nil(t, msg)
	require.Contains(t, err.Error(), "empty receipt was given")
	// Test case 2: Receipt with empty logs
	emptyReceipt := &types.Receipt{
		Logs: []*types.Log{},
	}
	msg, err = ExtractWarpMessageFromReceipt(emptyReceipt)
	require.Error(t, err)
	require.Nil(t, msg)
	require.Contains(t, err.Error(), "no warp message is present in evm logs")
	// Test case 3: Receipt with invalid warp message
	invalidPayload := []byte{1, 2, 3, 4, 5}
	invalidLog := &types.Log{
		Data: invalidPayload,
	}
	invalidReceipt := &types.Receipt{
		Logs: []*types.Log{invalidLog},
	}
	msg, err = ExtractWarpMessageFromReceipt(invalidReceipt)
	require.Error(t, err)
	require.Nil(t, msg)
	require.Contains(t, err.Error(), "no warp message is present in evm logs")
	// Test case 4: Receipt with valid warp message
	unsignedWarpMessage, err := warp.NewUnsignedMessage(
		0,
		ids.ID{},
		[]byte{},
	)
	require.NoError(t, err)
	_, validPayload, err := subnetevmwarp.PackSendWarpMessageEvent(
		common.Address{},
		common.Hash{},
		unsignedWarpMessage.Bytes(),
	)
	require.NoError(t, err)
	validLog := &types.Log{
		Data: validPayload,
	}
	validReceipt := &types.Receipt{
		Logs: []*types.Log{validLog},
	}
	msg, err = ExtractWarpMessageFromReceipt(validReceipt)
	require.NoError(t, err)
	require.Equal(t, unsignedWarpMessage, msg)
	// Test case 5: Receipt with multiple logs (valid and invalid)
	mixedReceipt := &types.Receipt{
		Logs: []*types.Log{validLog, invalidLog},
	}
	msg, err = ExtractWarpMessageFromReceipt(mixedReceipt)
	require.NoError(t, err)
	require.Equal(t, unsignedWarpMessage, msg)
}
