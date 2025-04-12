// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	subnetethclient "github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/rpc"
	"github.com/stretchr/testify/require"
)

func TestGetRawClient(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	// Save original function to restore later
	originalRPCDialContext := rpcDialContext
	defer func() {
		rpcDialContext = originalRPCDialContext
	}()
	originalDialContext := ethclientDialContext
	defer func() {
		ethclientDialContext = originalDialContext
	}()
	failuresCount := 0
	tests := []struct {
		name            string
		rpcURL          string
		mockRPCDialFunc func(context.Context, string) (*rpc.Client, error)
		mockDialFunc    func(context.Context, string) (subnetethclient.Client, error)
		expectError     bool
	}{
		{
			name:        "invalid url",
			rpcURL:      "http://:invalid",
			expectError: true,
		},
		{
			name:   "total failure with scheme",
			rpcURL: "http://localhost:8545",
			mockRPCDialFunc: func(_ context.Context, _ string) (*rpc.Client, error) {
				failuresCount++
				if failuresCount <= repeatsOnFailure {
					return nil, errors.New("connection error")
				}
				return &rpc.Client{}, nil
			},
			expectError: true,
		},
		{
			name:   "with scheme, 2 failures",
			rpcURL: "http://localhost:8545",
			mockRPCDialFunc: func(_ context.Context, _ string) (*rpc.Client, error) {
				failuresCount++
				if failuresCount < repeatsOnFailure {
					return nil, errors.New("connection error")
				}
				return &rpc.Client{}, nil
			},
			expectError: false,
		},
		{
			name:   "successful connection with scheme",
			rpcURL: "http://localhost:8545",
			mockRPCDialFunc: func(_ context.Context, _ string) (*rpc.Client, error) {
				return &rpc.Client{}, nil
			},
			expectError: false,
		},
		{
			name:   "without scheme, can't get scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, _ string) (subnetethclient.Client, error) {
				return nil, errors.New("invalid")
			},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the dial function with our mock
			rpcDialContext = tt.mockRPCDialFunc
			ethclientDialContext = tt.mockDialFunc
			failuresCount = 0
			client, err := GetRawClient(tt.rpcURL)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.rpcURL)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.rpcURL, client.URL)
				require.NotNil(t, client.RPCClient)
				require.NotNil(t, client.CallContext)
			}
		})
	}
}

func TestDebugTraceTransaction(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	failuresCount := 0
	tests := []struct {
		name        string
		txID        string
		mockCall    func(context.Context, interface{}, string, ...interface{}) error
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name: "successful trace",
			txID: "0x123",
			mockCall: func(_ context.Context, result interface{}, _ string, _ ...interface{}) error {
				// Cast result to the expected type and set the mock response
				if trace, ok := result.(*map[string]interface{}); ok {
					*trace = map[string]interface{}{
						"output": "0x123456",
						"gas":    "0x21000",
					}
				}
				return nil
			},
			expected: map[string]interface{}{
				"output": "0x123456",
				"gas":    "0x21000",
			},
			expectError: false,
		},
		{
			name: "error in RPC call",
			txID: "0x123",
			mockCall: func(_ context.Context, result interface{}, _ string, _ ...interface{}) error {
				if failuresCount <= repeatsOnFailure {
					return errors.New("RPC error")
				}
				if trace, ok := result.(*map[string]interface{}); ok {
					*trace = map[string]interface{}{
						"output": "0x123456",
						"gas":    "0x21000",
					}
				}
				return nil
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "successful after max errors",
			txID: "0x123",
			mockCall: func(_ context.Context, result interface{}, _ string, _ ...interface{}) error {
				failuresCount++
				if failuresCount < repeatsOnFailure {
					return errors.New("RPC error")
				}
				if trace, ok := result.(*map[string]interface{}); ok {
					*trace = map[string]interface{}{
						"output": "0x123456",
						"gas":    "0x21000",
					}
				}
				return nil
			},
			expected: map[string]interface{}{
				"output": "0x123456",
				"gas":    "0x21000",
			},
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := RawClient{
				URL:         "http://localhost:8545",
				CallContext: tt.mockCall,
			}
			trace, err := client.DebugTraceTransaction(tt.txID)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failure tracing tx")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, trace)
			}
		})
	}
}

func TestDebugTraceCall(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	failuresCount := 0
	tests := []struct {
		name        string
		data        map[string]string
		mockCall    func(context.Context, interface{}, string, ...interface{}) error
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name: "successful trace",
			data: map[string]string{
				"from":  "0x123",
				"to":    "0x456",
				"input": "0x789",
			},
			mockCall: func(_ context.Context, result interface{}, _ string, _ ...interface{}) error {
				// Cast result to the expected type and set the mock response
				if trace, ok := result.(*map[string]interface{}); ok {
					*trace = map[string]interface{}{
						"output": "0x123456",
						"gas":    "0x21000",
						"failed": false,
					}
				}
				return nil
			},
			expected: map[string]interface{}{
				"output": "0x123456",
				"gas":    "0x21000",
				"failed": false,
			},
			expectError: false,
		},
		{
			name: "error in RPC call",
			data: map[string]string{
				"from":  "0x123",
				"to":    "0x456",
				"input": "0x789",
			},
			mockCall: func(_ context.Context, result interface{}, _ string, _ ...interface{}) error {
				if failuresCount <= repeatsOnFailure {
					return errors.New("RPC error")
				}
				if trace, ok := result.(*map[string]interface{}); ok {
					*trace = map[string]interface{}{
						"output": "0x123456",
						"gas":    "0x21000",
						"failed": false,
					}
				}
				return nil
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "successful after max errors",
			data: map[string]string{
				"from":  "0x123",
				"to":    "0x456",
				"input": "0x789",
			},
			mockCall: func(_ context.Context, result interface{}, _ string, _ ...interface{}) error {
				failuresCount++
				if failuresCount < repeatsOnFailure {
					return errors.New("RPC error")
				}
				if trace, ok := result.(*map[string]interface{}); ok {
					*trace = map[string]interface{}{
						"output": "0x123456",
						"gas":    "0x21000",
						"failed": false,
					}
				}
				return nil
			},
			expected: map[string]interface{}{
				"output": "0x123456",
				"gas":    "0x21000",
				"failed": false,
			},
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := RawClient{
				URL:         "http://localhost:8545",
				CallContext: tt.mockCall,
			}
			trace, err := client.DebugTraceCall(tt.data)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failure tracing call")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, trace)
			}
		})
	}
}

func TestGetFunctionSelector(t *testing.T) {
	tests := []struct {
		name        string
		functionSig string
		expected    string
	}{
		{
			name:        "transfer function",
			functionSig: "transfer(address,uint256)",
			expected:    "0xa9059cbb",
		},
		{
			name:        "approve function",
			functionSig: "approve(address,uint256)",
			expected:    "0x095ea7b3",
		},
		{
			name:        "transferFrom function",
			functionSig: "transferFrom(address,address,uint256)",
			expected:    "0x23b872dd",
		},
		{
			name:        "balanceOf function",
			functionSig: "balanceOf(address)",
			expected:    "0x70a08231",
		},
		{
			name:        "allowance function",
			functionSig: "allowance(address,address)",
			expected:    "0xdd62ed3e",
		},
		{
			name:        "empty function",
			functionSig: "",
			expected:    "0xc5d24601",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := GetFunctionSelector(tt.functionSig)
			require.True(t, strings.HasPrefix(selector, "0x"))
			require.Len(t, selector, 10) // 0x + 8 hex characters
			require.Equal(t, selector, tt.expected)
		})
	}
}

func TestGetErrorFromTrace(t *testing.T) {
	testErr := fmt.Errorf("transfer failed")
	tests := []struct {
		name                     string
		trace                    map[string]interface{}
		functionSignatureToError map[string]error
		mappedErr                error
		expectedErr              bool
		errContains              string
	}{
		{
			name:                     "empty trace",
			trace:                    map[string]interface{}{},
			functionSignatureToError: map[string]error{},
			expectedErr:              true,
			errContains:              "trace does not contain output field",
		},
		{
			name: "output is not string",
			trace: map[string]interface{}{
				"output": 5,
				"gas":    "0x21000",
				"failed": true,
			},
			expectedErr: true,
			errContains: "expected type string for trace output, got",
		},
		{
			name: "output is not hexa",
			trace: map[string]interface{}{
				"output": "pp",
				"gas":    "0x21000",
				"failed": true,
			},
			expectedErr: true,
			errContains: "failure decoding trace output",
		},
		{
			name: "output has not enough bytes",
			trace: map[string]interface{}{
				"output": "0x",
				"gas":    "0x21000",
				"failed": true,
			},
			expectedErr: true,
			errContains: "less than 4 bytes in trace output",
		},
		{
			name: "trace with known function selector",
			trace: map[string]interface{}{
				"output": "0xa9059cbb123456", // transfer(address,uint256) selector
				"gas":    "0x21000",
				"failed": true,
			},
			functionSignatureToError: map[string]error{
				"transfer(address,uint256)": testErr,
			},
			mappedErr:   testErr,
			expectedErr: false,
		},
		{
			name: "trace with unknown function selector",
			trace: map[string]interface{}{
				"output": "0xa9059cbb123456", // transfer(address,uint256) selector
				"gas":    "0x21000",
				"failed": true,
			},
			expectedErr: true,
			errContains: ErrUnknownErrorSelector.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mappedErr, err := GetErrorFromTrace(tt.trace, tt.functionSignatureToError)
			require.Equal(t, tt.mappedErr, mappedErr)
			if tt.expectedErr {
				require.Error(t, err)
				require.True(t, strings.Contains(err.Error(), tt.errContains))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
