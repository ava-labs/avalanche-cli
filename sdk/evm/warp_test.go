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
