// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ava-labs/subnet-evm/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestGetEventFromLogs(t *testing.T) {
	type parserResult struct {
		Value string
	}
	parser := func(log types.Log) (parserResult, error) {
		if string(log.Data) == "valid" {
			return parserResult{Value: "success"}, nil
		}
		return parserResult{}, errors.New("invalid log data")
	}
	tests := []struct {
		name        string
		logs        []*types.Log
		expectError bool
		expected    parserResult
	}{
		{
			name:        "empty logs",
			logs:        []*types.Log{},
			expectError: true,
		},
		{
			name: "no valid logs",
			logs: []*types.Log{
				{Data: []byte("invalid1")},
				{Data: []byte("invalid2")},
			},
			expectError: true,
		},
		{
			name: "valid log at start",
			logs: []*types.Log{
				{Data: []byte("valid")},
				{Data: []byte("invalid")},
			},
			expectError: false,
			expected:    parserResult{Value: "success"},
		},
		{
			name: "valid log at end",
			logs: []*types.Log{
				{Data: []byte("invalid")},
				{Data: []byte("valid")},
			},
			expectError: false,
			expected:    parserResult{Value: "success"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := GetEventFromLogs(tt.logs, parser)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failed to find evm.parserResult event in receipt logs")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, event)
			}
		})
	}
}

func TestTransactionError(t *testing.T) {
	tx := types.NewTransaction(0, common.Address{}, nil, 0, nil, nil)
	tests := []struct {
		name          string
		tx            *types.Transaction
		err           error
		msg           string
		args          []interface{}
		shouldContain string
	}{
		{
			name:          "with transaction, without formatting",
			tx:            tx,
			err:           errors.New("test error"),
			msg:           "test message",
			args:          nil,
			shouldContain: "test message",
		},
		{
			name:          "without transaction, with formatting",
			tx:            nil,
			err:           errors.New("test error"),
			msg:           "test message shows %d value",
			args:          []interface{}{11},
			shouldContain: "test message shows 11 value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TransactionError(tt.tx, tt.err, tt.msg, tt.args...)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.shouldContain)
			if tt.tx != nil {
				require.Contains(t, err.Error(), tt.tx.Hash().String())
			} else {
				require.Contains(t, err.Error(), "tx failed to be submitted")
			}
		})
	}
}

func TestTxDump(t *testing.T) {
	testData := []byte{1, 2, 3, 4, 5, 6}
	tx := types.NewTransaction(0, common.Address{}, nil, 0, nil, testData)
	testAddress := common.HexToAddress("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed")
	txWithAccessList := types.NewTx(&types.DynamicFeeTx{
		Data: testData,
		AccessList: types.AccessList{
			types.AccessTuple{
				Address: testAddress,
			},
		},
	})
	tests := []struct {
		name             string
		description      string
		tx               *types.Transaction
		expectAccessList bool
		expectError      bool
	}{
		{
			name:             "valid transaction with access list",
			description:      "test transaction with access list",
			tx:               txWithAccessList,
			expectError:      false,
			expectAccessList: true,
		},
		{
			name:             "valid transaction",
			description:      "test transaction",
			tx:               tx,
			expectError:      false,
			expectAccessList: false,
		},
		{
			name:        "nil transaction",
			description: "test transaction",
			tx:          nil,
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dump, err := TxDump(tt.description, tt.tx)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Contains(t, dump, "Tx Dump For")
				require.Contains(t, dump, tt.description)
				require.Contains(t, dump, "Calldata Dump")
				require.Contains(t, dump, hex.EncodeToString(testData))
				if tt.expectAccessList {
					require.Contains(t, dump, "Access List Dump")
					require.Contains(t, dump, "Address: "+testAddress.Hex())
				} else {
					require.NotContains(t, dump, "Access List Dump")
				}
			}
		})
	}
}

func TestPrivateKeyToAddress(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	expectedAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	tests := []struct {
		name        string
		privateKey  string
		expectError bool
		expected    common.Address
	}{
		{
			name:        "valid private key",
			privateKey:  privateKeyHex,
			expectError: false,
			expected:    expectedAddress,
		},
		{
			name:        "invalid private key",
			privateKey:  "invalid",
			expectError: true,
		},
		{
			name:        "empty private key",
			privateKey:  "",
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, err := PrivateKeyToAddress(tt.privateKey)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, address)
			}
		})
	}
}
