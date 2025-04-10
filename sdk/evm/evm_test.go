// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package evm

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/sdk/constants"
	mockethclient "github.com/ava-labs/avalanche-cli/sdk/mocks/ethclient"
	avalancheWarp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/subnet-evm/core/types"
	subnetethclient "github.com/ava-labs/subnet-evm/ethclient"
	"github.com/ava-labs/subnet-evm/interfaces"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHasScheme(t *testing.T) {
	tests := []struct {
		name     string
		rpcURL   string
		expected bool
		errors   bool
	}{
		{
			name:     "with http scheme",
			rpcURL:   "http://localhost:8545",
			expected: true,
		},
		{
			name:     "with https scheme",
			rpcURL:   "https://localhost:8545",
			expected: true,
		},
		{
			name:     "with ws scheme",
			rpcURL:   "ws://localhost:8545",
			expected: true,
		},
		{
			name:     "with wss scheme",
			rpcURL:   "wss://localhost:8545",
			expected: true,
		},
		{
			name:     "without scheme",
			rpcURL:   "localhost:8545",
			expected: false,
		},
		{
			name:     "IP without scheme",
			rpcURL:   "127.0.0.1:123",
			expected: false,
		},
		{
			name:     "IP with scheme",
			rpcURL:   "ws://127.0.0.1:123",
			expected: true,
		},
		{
			name:     "invalid url",
			rpcURL:   "http://:invalid",
			expected: false,
			errors:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasScheme, err := HasScheme(tt.rpcURL)
			if tt.errors {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, hasScheme)
			}
		})
	}
}

func TestGetClientWithoutScheme(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	// Save original function to restore later
	originalDialContext := ethclientDialContext
	defer func() {
		ethclientDialContext = originalDialContext
	}()
	tests := []struct {
		name           string
		rpcURL         string
		mockDialFunc   func(context.Context, string) (subnetethclient.Client, error)
		expectedScheme string
		expectError    bool
	}{
		{
			name:        "invalid url",
			rpcURL:      "http://:invalid",
			expectError: true,
		},
		{
			name:   "success with ws scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, url string) (subnetethclient.Client, error) {
				if strings.HasPrefix(url, "ws://") {
					return mockethclient.NewMockClient(gomock.NewController(t)), nil
				}
				return nil, errors.New("invalid")
			},
			expectedScheme: "ws://",
			expectError:    false,
		},
		{
			name:   "failure with ws scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, url string) (subnetethclient.Client, error) {
				if strings.HasPrefix(url, "ws://") {
					return nil, errors.New("unexpected error on ws connection")
				}
				return nil, errors.New("invalid")
			},
			expectError: true,
		},
		{
			name:   "success with wss scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, url string) (subnetethclient.Client, error) {
				if strings.HasPrefix(url, "ws://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "wss://") {
					return mockethclient.NewMockClient(gomock.NewController(t)), nil
				}
				return nil, errors.New("invalid")
			},
			expectedScheme: "wss://",
			expectError:    false,
		},
		{
			name:   "failure with wss scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, url string) (subnetethclient.Client, error) {
				if strings.HasPrefix(url, "ws://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "wss://") {
					return nil, errors.New("unexpected error on wss connection")
				}
				return nil, errors.New("invalid")
			},
			expectError: true,
		},
		{
			name:   "success with https scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, url string) (subnetethclient.Client, error) {
				if strings.HasPrefix(url, "ws://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "wss://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "https://") {
					mockClient := mockethclient.NewMockClient(gomock.NewController(t))
					mockClient.EXPECT().ChainID(gomock.Any()).Return(big.NewInt(1), nil)
					return mockClient, nil
				}
				return nil, errors.New("invalid")
			},
			expectedScheme: "https://",
			expectError:    false,
		},
		{
			name:   "failure with https scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, url string) (subnetethclient.Client, error) {
				if strings.HasPrefix(url, "ws://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "wss://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "https://") {
					mockClient := mockethclient.NewMockClient(gomock.NewController(t))
					mockClient.EXPECT().ChainID(gomock.Any()).Return(big.NewInt(1), errors.New("unexpected error on https connection"))
					return mockClient, nil
				}
				return nil, errors.New("invalid")
			},
			expectError: true,
		},
		{
			name:   "success with http scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, url string) (subnetethclient.Client, error) {
				if strings.HasPrefix(url, "ws://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "wss://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "https://") {
					mockClient := mockethclient.NewMockClient(gomock.NewController(t))
					mockClient.EXPECT().ChainID(gomock.Any()).Return(big.NewInt(1), errors.New("server gave HTTP response to HTTPS client"))
					return mockClient, nil
				}
				if strings.HasPrefix(url, "http://") {
					return mockethclient.NewMockClient(gomock.NewController(t)), nil
				}
				return nil, errors.New("invalid")
			},
			expectedScheme: "http://",
			expectError:    false,
		},
		{
			name:   "error - url with scheme",
			rpcURL: "http://localhost:8545",
			mockDialFunc: func(_ context.Context, _ string) (subnetethclient.Client, error) {
				return nil, nil
			},
			expectError: true,
		},
		{
			name:   "error - unknown protocol",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, _ string) (subnetethclient.Client, error) {
				return nil, errors.New("unknown protocol")
			},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the dial function with our mock
			ethclientDialContext = tt.mockDialFunc
			client, scheme, err := GetClientWithoutScheme(tt.rpcURL)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
				require.Equal(t, tt.expectedScheme, scheme)
			}
		})
	}
}

func TestGetClient(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	// Save original function to restore later
	originalDialContext := ethclientDialContext
	defer func() {
		ethclientDialContext = originalDialContext
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	failuresCount := 0
	tests := []struct {
		name         string
		rpcURL       string
		mockDialFunc func(context.Context, string) (subnetethclient.Client, error)
		expectError  bool
	}{
		{
			name:        "invalid url",
			rpcURL:      "http://:invalid",
			expectError: true,
		},
		{
			name:   "with scheme, total failure",
			rpcURL: "http://localhost:8545",
			mockDialFunc: func(_ context.Context, _ string) (subnetethclient.Client, error) {
				failuresCount++
				if failuresCount <= repeatsOnFailure {
					return nil, errors.New("connection error")
				}
				return mockethclient.NewMockClient(ctrl), nil
			},
			expectError: true,
		},
		{
			name:   "with scheme, 2 failures",
			rpcURL: "http://localhost:8545",
			mockDialFunc: func(_ context.Context, _ string) (subnetethclient.Client, error) {
				failuresCount++
				if failuresCount < repeatsOnFailure {
					return nil, errors.New("connection error")
				}
				return mockethclient.NewMockClient(ctrl), nil
			},
			expectError: false,
		},
		{
			name:   "with scheme",
			rpcURL: "http://localhost:8545",
			mockDialFunc: func(_ context.Context, _ string) (subnetethclient.Client, error) {
				return mockethclient.NewMockClient(ctrl), nil
			},
			expectError: false,
		},
		{
			name:   "without scheme",
			rpcURL: "localhost:8545",
			mockDialFunc: func(_ context.Context, url string) (subnetethclient.Client, error) {
				if strings.HasPrefix(url, "ws://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "wss://") {
					return nil, errors.New("websocket: bad handshake")
				}
				if strings.HasPrefix(url, "https://") {
					mockClient := mockethclient.NewMockClient(gomock.NewController(t))
					mockClient.EXPECT().ChainID(gomock.Any()).Return(big.NewInt(1), errors.New("server gave HTTP response to HTTPS client"))
					return mockClient, nil
				}
				if strings.HasPrefix(url, "http://") {
					return mockethclient.NewMockClient(gomock.NewController(t)), nil
				}
				return nil, errors.New("invalid")
			},
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the dial function with our mock
			ethclientDialContext = tt.mockDialFunc
			failuresCount = 0
			client, err := GetClient(tt.rpcURL)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.rpcURL)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.rpcURL, client.URL)
			}
		})
	}
}

func TestClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	// Expect Close to be called exactly once
	mockClient.EXPECT().Close().Times(1)
	// Call Close
	client.Close()
	// Verify all expectations were met
	ctrl.Finish()
}

func TestContractAlreadyDeployed(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name         string
		contractAddr string
		setupMock    func()
		expected     bool
		expectError  bool
	}{
		{
			name:         "contract deployed",
			contractAddr: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				mockClient.EXPECT().CodeAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]byte{1, 2, 3}, nil)
			},
			expected:    true,
			expectError: false,
		},
		{
			name:         "contract not deployed",
			contractAddr: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				mockClient.EXPECT().CodeAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]byte{}, nil)
			},
			expected:    false,
			expectError: false,
		},
		{
			name:         "error getting code",
			contractAddr: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().CodeAt(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("failed to get code"))
				}
			},
			expected:    false,
			expectError: true,
		},
		{
			name:         "getting code after max failues",
			contractAddr: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().CodeAt(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("failed to get code"))
				}
				mockClient.EXPECT().CodeAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]byte{1, 2, 3}, nil)
			},
			expected:    true,
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			deployed, err := client.ContractAlreadyDeployed(tt.contractAddr)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, deployed)
			}
		})
	}
}

func TestGetAddressBalance(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name        string
		address     string
		setupMock   func()
		expected    *big.Int
		expectError bool
	}{
		{
			name:    "successful balance check",
			address: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				mockClient.EXPECT().BalanceAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(big.NewInt(1000), nil)
			},
			expected:    big.NewInt(1000),
			expectError: false,
		},
		{
			name:    "error getting balance",
			address: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().BalanceAt(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("failed to get balance"))
				}
			},
			expected:    nil,
			expectError: true,
		},
		{
			name:    "successful balance check after max failures",
			address: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().BalanceAt(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(nil, errors.New("failed to get balance"))
				}
				mockClient.EXPECT().BalanceAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(big.NewInt(1000), nil)
			},
			expected:    big.NewInt(1000),
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			balance, err := client.GetAddressBalance(tt.address)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.address)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, balance)
			}
		})
	}
}

func TestNonceAt(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name        string
		address     string
		setupMock   func()
		expected    uint64
		expectError bool
	}{
		{
			name:    "successful nonce check",
			address: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				mockClient.EXPECT().NonceAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(uint64(42), nil)
			},
			expected:    42,
			expectError: false,
		},
		{
			name:    "error getting nonce",
			address: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().NonceAt(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(uint64(0), errors.New("failed to get nonce"))
				}
			},
			expected:    0,
			expectError: true,
		},
		{
			name:    "successful after max failures",
			address: "0x1234567890123456789012345678901234567890",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().NonceAt(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(uint64(0), errors.New("failed to get nonce"))
				}
				mockClient.EXPECT().NonceAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(uint64(42), nil)
			},
			expected:    42,
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			nonce, err := client.NonceAt(tt.address)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.address)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, nonce)
			}
		})
	}
}

func TestSuggestGasTipCap(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name        string
		setupMock   func()
		expected    *big.Int
		expectError bool
	}{
		{
			name: "successful gas tip cap suggestion",
			setupMock: func() {
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
			},
			expected:    big.NewInt(1000000000),
			expectError: false,
		},
		{
			name: "error getting gas tip cap",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
						Return(nil, errors.New("failed to get gas tip cap"))
				}
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "successful after max failures",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
						Return(nil, errors.New("failed to get gas tip cap"))
				}
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(100), nil)
			},
			expected:    big.NewInt(100),
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			gasTipCap, err := client.SuggestGasTipCap()
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "gas tip cap")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, gasTipCap)
			}
		})
	}
}

func TestEstimateBaseFee(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name        string
		setupMock   func()
		expected    *big.Int
		expectError bool
	}{
		{
			name: "successful base fee estimation",
			setupMock: func() {
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
			},
			expected:    big.NewInt(10000000000),
			expectError: false,
		},
		{
			name: "error estimating base fee",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
						Return(nil, errors.New("failed to estimate base fee"))
				}
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "successful after max failures",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
						Return(nil, errors.New("failed to estimate base fee"))
				}
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(100), nil)
			},
			expected:    big.NewInt(100),
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			baseFee, err := client.EstimateBaseFee()
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "base fee")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, baseFee)
			}
		})
	}
}

func TestEstimateGasLimit(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name        string
		setupMock   func()
		expected    uint64
		expectError bool
	}{
		{
			name: "successful gas limit estimation",
			setupMock: func() {
				mockClient.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).
					Return(uint64(21000), nil)
			},
			expected:    21000,
			expectError: false,
		},
		{
			name: "error estimating gas limit",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).
						Return(uint64(0), errors.New("failed to estimate gas"))
				}
			},
			expected:    0,
			expectError: true,
		},
		{
			name: "successful after max failures",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).
						Return(uint64(0), errors.New("failed to estimate gas"))
				}
				mockClient.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).
					Return(uint64(21000), nil)
			},
			expected:    21000,
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			gasLimit, err := client.EstimateGasLimit(interfaces.CallMsg{})
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "gas limit")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, gasLimit)
			}
		})
	}
}

func TestGetChainID(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name        string
		setupMock   func()
		expected    *big.Int
		expectError bool
	}{
		{
			name: "successful chain ID retrieval",
			setupMock: func() {
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
			},
			expected:    big.NewInt(43114),
			expectError: false,
		},
		{
			name: "error getting chain ID",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(nil, errors.New("failed to get chain ID"))
				}
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "successful after max failures",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(nil, errors.New("failed to get chain ID"))
				}
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
			},
			expected:    big.NewInt(43114),
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			chainID, err := client.GetChainID()
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "chain id")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, chainID)
			}
		})
	}
}

func TestSendTransaction(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tx := types.NewTransaction(0, common.Address{}, nil, 0, nil, nil)
	tests := []struct {
		name        string
		setupMock   func()
		expectError bool
	}{
		{
			name: "successful transaction send",
			setupMock: func() {
				mockClient.EXPECT().SendTransaction(gomock.Any(), tx).
					Return(nil)
			},
			expectError: false,
		},
		{
			name: "error sending transaction",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().SendTransaction(gomock.Any(), tx).
						Return(errors.New("failed to send transaction"))
				}
			},
			expectError: true,
		},
		{
			name: "successful after max failures",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().SendTransaction(gomock.Any(), tx).
						Return(errors.New("failed to send transaction"))
				}
				mockClient.EXPECT().SendTransaction(gomock.Any(), tx).
					Return(nil)
			},
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			err := client.SendTransaction(tx)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "sending transaction")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWaitForTransaction(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tx := types.NewTransaction(0, common.Address{}, nil, 0, nil, nil)
	successfulReceipt := &types.Receipt{Status: types.ReceiptStatusSuccessful}
	failedReceipt := &types.Receipt{Status: types.ReceiptStatusFailed}
	tests := []struct {
		name        string
		setupMock   func()
		expected    *types.Receipt
		success     bool
		expectError bool
	}{
		{
			name: "successful transaction",
			setupMock: func() {
				mockClient.EXPECT().TransactionReceipt(gomock.Any(), tx.Hash()).
					Return(successfulReceipt, nil)
			},
			expected:    successfulReceipt,
			success:     true,
			expectError: false,
		},
		{
			name: "failed transaction",
			setupMock: func() {
				mockClient.EXPECT().TransactionReceipt(gomock.Any(), tx.Hash()).
					Return(failedReceipt, nil)
			},
			expected:    failedReceipt,
			success:     false,
			expectError: false,
		},
		{
			name: "error waiting for transaction",
			setupMock: func() {
				steps := int(constants.APIRequestLargeTimeout.Seconds())
				for i := 0; i < steps*repeatsOnFailure; i++ {
					mockClient.EXPECT().TransactionReceipt(gomock.Any(), tx.Hash()).
						Return(nil, errors.New("failed to get receipt"))
				}
			},
			expected:    nil,
			success:     false,
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			receipt, success, err := client.WaitForTransaction(tx)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "waiting for tx")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, receipt)
				require.Equal(t, tt.success, success)
			}
		})
	}
}

func TestBlockByNumber(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	blockNumber := big.NewInt(1)
	header := &types.Header{
		Number: blockNumber,
	}
	block := types.NewBlock(header, nil, nil, nil, nil)
	tests := []struct {
		name        string
		setupMock   func()
		expected    *types.Block
		expectError bool
	}{
		{
			name: "successful block retrieval",
			setupMock: func() {
				mockClient.EXPECT().BlockByNumber(gomock.Any(), blockNumber).
					Return(block, nil)
			},
			expected:    block,
			expectError: false,
		},
		{
			name: "error getting block",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().BlockByNumber(gomock.Any(), blockNumber).
						Return(nil, errors.New("failed to get block"))
				}
			},
			expected:    nil,
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			result, err := client.BlockByNumber(blockNumber)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "retrieving block")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFilterLogs(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	logs := []types.Log{
		{Address: common.HexToAddress("0x123")},
		{Address: common.HexToAddress("0x456")},
	}
	tests := []struct {
		name        string
		setupMock   func()
		expected    []types.Log
		expectError bool
	}{
		{
			name: "successful log filtering",
			setupMock: func() {
				mockClient.EXPECT().FilterLogs(gomock.Any(), gomock.Any()).
					Return(logs, nil)
			},
			expected:    logs,
			expectError: false,
		},
		{
			name: "error filtering logs",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().FilterLogs(gomock.Any(), gomock.Any()).
						Return(nil, errors.New("failed to filter logs"))
				}
			},
			expected:    nil,
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			result, err := client.FilterLogs(interfaces.FilterQuery{})
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "retrieving logs")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTransactionReceipt(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	hash := common.HexToHash("0x123")
	receipt := &types.Receipt{Status: types.ReceiptStatusSuccessful}
	tests := []struct {
		name        string
		setupMock   func()
		expected    *types.Receipt
		expectError bool
	}{
		{
			name: "successful receipt retrieval",
			setupMock: func() {
				mockClient.EXPECT().TransactionReceipt(gomock.Any(), hash).
					Return(receipt, nil)
			},
			expected:    receipt,
			expectError: false,
		},
		{
			name: "error getting receipt",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().TransactionReceipt(gomock.Any(), hash).
						Return(nil, errors.New("failed to get receipt"))
				}
			},
			expected:    nil,
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			result, err := client.TransactionReceipt(hash)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "retrieving receipt")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestBlockNumber(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name        string
		setupMock   func()
		expected    uint64
		expectError bool
	}{
		{
			name: "successful block number retrieval",
			setupMock: func() {
				mockClient.EXPECT().BlockNumber(gomock.Any()).
					Return(uint64(1000), nil)
			},
			expected:    1000,
			expectError: false,
		},
		{
			name: "error getting block number",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().BlockNumber(gomock.Any()).
						Return(uint64(0), errors.New("failed to get block number"))
				}
			},
			expected:    0,
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			result, err := client.BlockNumber()
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "retrieving height")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetPrivateKeyBalance(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	tests := []struct {
		name        string
		privateKey  string
		setupMock   func()
		expected    *big.Int
		expectError bool
	}{
		{
			name:       "successful balance check",
			privateKey: privateKeyHex,
			setupMock: func() {
				mockClient.EXPECT().BalanceAt(gomock.Any(), address, gomock.Any()).
					Return(big.NewInt(1000), nil)
			},
			expected:    big.NewInt(1000),
			expectError: false,
		},
		{
			name:       "error getting balance",
			privateKey: privateKeyHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().BalanceAt(gomock.Any(), address, gomock.Any()).
						Return(nil, errors.New("failed to get balance"))
				}
			},
			expected:    nil,
			expectError: true,
		},
		{
			name:       "successful after max failures",
			privateKey: privateKeyHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().BalanceAt(gomock.Any(), address, gomock.Any()).
						Return(nil, errors.New("failed to get balance"))
				}
				mockClient.EXPECT().BalanceAt(gomock.Any(), address, gomock.Any()).
					Return(big.NewInt(100), nil)
			},
			expected:    big.NewInt(100),
			expectError: false,
		},
		{
			name:        "invalid private key",
			privateKey:  "invalid",
			setupMock:   func() {},
			expected:    nil,
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			balance, err := client.GetPrivateKeyBalance(tt.privateKey)
			if tt.expectError {
				require.Error(t, err)
				if tt.privateKey != "invalid" {
					require.Contains(t, err.Error(), "obtaining balance")
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, balance)
			}
		})
	}
}

func TestCalculateTxParams(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")
	tests := []struct {
		name          string
		address       string
		setupMock     func()
		expectedFee   *big.Int
		expectedTip   *big.Int
		expectedNonce uint64
		expectError   bool
	}{
		{
			name:    "successful calculation",
			address: address.Hex(),
			setupMock: func() {
				// Base fee estimation
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				// Gas tip cap suggestion
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				// Nonce retrieval
				mockClient.EXPECT().NonceAt(gomock.Any(), address, gomock.Any()).
					Return(uint64(42), nil)
			},
			expectedFee:   big.NewInt(22500000000),
			expectedTip:   big.NewInt(1000000000),
			expectedNonce: 42,
			expectError:   false,
		},
		{
			name:    "error estimating base fee",
			address: address.Hex(),
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
						Return(nil, errors.New("failed to estimate base fee"))
				}
			},
			expectError: true,
		},
		{
			name:    "error suggesting gas tip cap",
			address: address.Hex(),
			setupMock: func() {
				// Base fee estimation succeeds
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				// Gas tip cap suggestion fails
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
						Return(nil, errors.New("failed to suggest gas tip cap"))
				}
			},
			expectError: true,
		},
		{
			name:    "error getting nonce",
			address: address.Hex(),
			setupMock: func() {
				// Base fee estimation succeeds
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				// Gas tip cap suggestion succeeds
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				// Nonce retrieval fails
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().NonceAt(gomock.Any(), address, gomock.Any()).
						Return(uint64(0), errors.New("failed to get nonce"))
				}
			},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			gasFeeCap, gasTipCap, nonce, err := client.CalculateTxParams(tt.address)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failure")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedFee, gasFeeCap)
				require.Equal(t, tt.expectedTip, gasTipCap)
				require.Equal(t, tt.expectedNonce, nonce)
			}
		})
	}
}

func TestFundAddress(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	sourceAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	targetAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	amount := big.NewInt(1000000000000000000) // 1 ETH
	tests := []struct {
		name        string
		setupMock   func()
		expectError bool
	}{
		{
			name: "successful fund transfer",
			setupMock: func() {
				// CalculateTxParams
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), sourceAddress, gomock.Any()).
					Return(uint64(42), nil)
				// GetChainID
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
				// SendTransaction
				mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
					Return(nil)
				// TransactionReceipt
				mockClient.EXPECT().TransactionReceipt(gomock.Any(), gomock.Any()).
					Return(&types.Receipt{Status: types.ReceiptStatusSuccessful}, nil)
			},
			expectError: false,
		},
		{
			name: "error calculating tx params",
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
						Return(nil, errors.New("failed to estimate base fee"))
				}
			},
			expectError: true,
		},
		{
			name: "error getting chain ID",
			setupMock: func() {
				// CalculateTxParams succeeds
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), sourceAddress, gomock.Any()).
					Return(uint64(42), nil)
				// GetChainID fails
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(nil, errors.New("failed to get chain ID"))
				}
			},
			expectError: true,
		},
		{
			name: "error sending transaction",
			setupMock: func() {
				// CalculateTxParams succeeds
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), sourceAddress, gomock.Any()).
					Return(uint64(42), nil)
				// GetChainID succeeds
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
				// SendTransaction fails
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
						Return(errors.New("failed to send transaction"))
				}
			},
			expectError: true,
		},
		{
			name: "transaction failed",
			setupMock: func() {
				// CalculateTxParams succeeds
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), sourceAddress, gomock.Any()).
					Return(uint64(42), nil)
				// GetChainID succeeds
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
				// SendTransaction succeeds
				mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
					Return(nil)
				// TransactionReceipt returns failed status
				mockClient.EXPECT().TransactionReceipt(gomock.Any(), gomock.Any()).
					Return(&types.Receipt{Status: types.ReceiptStatusFailed}, nil)
			},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			err := client.FundAddress(privateKeyHex, targetAddress.Hex(), amount)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failure")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIssueTx(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tx := types.NewTransaction(0, common.Address{}, nil, 0, nil, nil)
	txBytes, err := tx.MarshalBinary()
	require.NoError(t, err)
	txHex := hex.EncodeToString(txBytes)
	tests := []struct {
		name        string
		txStr       string
		setupMock   func()
		expectError bool
	}{
		{
			name:  "successful transaction",
			txStr: txHex,
			setupMock: func() {
				// SendTransaction
				mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
					Return(nil)
				// TransactionReceipt
				mockClient.EXPECT().TransactionReceipt(gomock.Any(), gomock.Any()).
					Return(&types.Receipt{Status: types.ReceiptStatusSuccessful}, nil)
			},
			expectError: false,
		},
		{
			name:        "invalid transaction hex",
			txStr:       "invalid",
			setupMock:   func() {},
			expectError: true,
		},
		{
			name:  "error sending transaction",
			txStr: txHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
						Return(errors.New("failed to send transaction"))
				}
			},
			expectError: true,
		},
		{
			name:  "transaction failed",
			txStr: txHex,
			setupMock: func() {
				// SendTransaction succeeds
				mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
					Return(nil)
				// TransactionReceipt returns failed status
				mockClient.EXPECT().TransactionReceipt(gomock.Any(), gomock.Any()).
					Return(&types.Receipt{Status: types.ReceiptStatusFailed}, nil)
			},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			err := client.IssueTx(tt.txStr)
			if tt.expectError {
				require.Error(t, err)
				if tt.name != "invalid transaction hex" {
					require.Contains(t, err.Error(), "failure")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetTxOptsWithSigner(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	tests := []struct {
		name        string
		privateKey  string
		setupMock   func()
		expectError bool
	}{
		{
			name:       "successful signer creation",
			privateKey: privateKeyHex,
			setupMock: func() {
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
			},
			expectError: false,
		},
		{
			name:        "invalid private key",
			privateKey:  "invalid",
			setupMock:   func() {},
			expectError: true,
		},
		{
			name:       "error getting chain ID",
			privateKey: privateKeyHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(nil, errors.New("failed to get chain ID"))
				}
			},
			expectError: true,
		},
		{
			name:       "successful after max failures",
			privateKey: privateKeyHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure-1; i++ {
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(nil, errors.New("failed to get chain ID"))
				}
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
			},
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			opts, err := client.GetTxOptsWithSigner(tt.privateKey)
			if tt.expectError {
				require.Error(t, err)
				if tt.privateKey != "invalid" {
					require.Contains(t, err.Error(), "failure generating signer")
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, opts)
				require.NotNil(t, opts.Signer)
			}
		})
	}
}

func TestWaitForEVMBootstrapped(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	tests := []struct {
		name        string
		timeout     time.Duration
		setupMock   func()
		expectError bool
	}{
		{
			name:    "successful bootstrap",
			timeout: 1 * time.Second,
			setupMock: func() {
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
			},
			expectError: false,
		},
		{
			name:    "timeout waiting for bootstrap",
			timeout: 100 * time.Millisecond,
			setupMock: func() {
				// Simulate multiple failures until timeout
				for i := 0; i < 10; i++ {
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(nil, errors.New("not bootstrapped"))
				}
			},
			expectError: true,
		},
		{
			name:    "default timeout",
			timeout: 0,
			setupMock: func() {
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
			},
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			err := client.WaitForEVMBootstrapped(tt.timeout)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "not bootstrapped")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTransactWithWarpMessage(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	contractAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	callData := []byte{1, 2, 3, 4, 5}
	value := big.NewInt(1000000000000000000) // 1 ETH
	unsignedMessage := avalancheWarp.UnsignedMessage{
		SourceChainID: [32]byte{1, 2, 3},
		Payload:       []byte{4, 5, 6},
	}
	warpMessage := &avalancheWarp.Message{
		UnsignedMessage: unsignedMessage,
	}
	tests := []struct {
		name              string
		from              common.Address
		privateKey        string
		warpMessage       *avalancheWarp.Message
		contract          common.Address
		callData          []byte
		value             *big.Int
		generateRawTxOnly bool
		setupMock         func()
		expectError       bool
	}{
		{
			name:              "successful transaction with private key",
			from:              common.Address{},
			privateKey:        privateKeyHex,
			warpMessage:       warpMessage,
			contract:          contractAddress,
			callData:          callData,
			value:             value,
			generateRawTxOnly: false,
			setupMock: func() {
				// CalculateTxParams
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), fromAddress, gomock.Any()).
					Return(uint64(42), nil)
				// GetChainID
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
				// EstimateGasLimit
				mockClient.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).
					Return(uint64(21000), nil)
			},
			expectError: false,
		},
		{
			name:              "successful raw transaction with from address",
			from:              fromAddress,
			privateKey:        "",
			warpMessage:       warpMessage,
			contract:          contractAddress,
			callData:          callData,
			value:             value,
			generateRawTxOnly: true,
			setupMock: func() {
				// CalculateTxParams
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), fromAddress, gomock.Any()).
					Return(uint64(42), nil)
				// GetChainID
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
				// EstimateGasLimit
				mockClient.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).
					Return(uint64(21000), nil)
			},
			expectError: false,
		},
		{
			name:              "error calculating tx params",
			from:              common.Address{},
			privateKey:        privateKeyHex,
			warpMessage:       warpMessage,
			contract:          contractAddress,
			callData:          callData,
			value:             value,
			generateRawTxOnly: false,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
						Return(nil, errors.New("failed to estimate base fee"))
				}
			},
			expectError: true,
		},
		{
			name:              "error getting chain ID",
			from:              common.Address{},
			privateKey:        privateKeyHex,
			warpMessage:       warpMessage,
			contract:          contractAddress,
			callData:          callData,
			value:             value,
			generateRawTxOnly: false,
			setupMock: func() {
				// CalculateTxParams succeeds
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), fromAddress, gomock.Any()).
					Return(uint64(42), nil)
				// GetChainID fails
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(nil, errors.New("failed to get chain ID"))
				}
			},
			expectError: true,
		},
		{
			name:              "error estimating gas limit",
			from:              common.Address{},
			privateKey:        privateKeyHex,
			warpMessage:       warpMessage,
			contract:          contractAddress,
			callData:          callData,
			value:             value,
			generateRawTxOnly: false,
			setupMock: func() {
				// CalculateTxParams succeeds
				mockClient.EXPECT().EstimateBaseFee(gomock.Any()).
					Return(big.NewInt(10000000000), nil)
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(big.NewInt(1000000000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), fromAddress, gomock.Any()).
					Return(uint64(42), nil)
				// GetChainID succeeds
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(big.NewInt(43114), nil)
				// EstimateGasLimit fails
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().EstimateGas(gomock.Any(), gomock.Any()).
						Return(uint64(0), errors.New("failed to estimate gas"))
				}
			},
			expectError: false, // Should use default gas limit
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			tx, err := client.TransactWithWarpMessage(
				tt.from,
				tt.privateKey,
				tt.warpMessage,
				tt.contract,
				tt.callData,
				tt.value,
				tt.generateRawTxOnly,
			)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failure")
			} else {
				require.NoError(t, err)
				require.NotNil(t, tx)
				if !tt.generateRawTxOnly {
					require.NotNil(t, tx.Hash())
				}
			}
		})
	}
}

func TestWaitForNewBlock(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	initialBlock := uint64(1000)
	newBlock := uint64(1001)
	tests := []struct {
		name            string
		prevBlockNumber uint64
		totalDuration   time.Duration
		setupMock       func()
		expectError     bool
	}{
		{
			name:            "successful block retrieval",
			prevBlockNumber: initialBlock,
			setupMock: func() {
				// new block found
				mockClient.EXPECT().BlockNumber(gomock.Any()).
					Return(newBlock, nil)
			},
			expectError: false,
		},
		{
			name:            "timeout waiting for new block",
			prevBlockNumber: initialBlock,
			setupMock: func() {
				// Simulate multiple checks with same block number until timeout
				for i := 0; i < 10; i++ {
					mockClient.EXPECT().BlockNumber(gomock.Any()).
						Return(initialBlock, nil)
				}
			},
			expectError: true,
		},
		{
			name:            "error getting block number",
			prevBlockNumber: initialBlock,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					mockClient.EXPECT().BlockNumber(gomock.Any()).
						Return(uint64(0), errors.New("failed to get block number"))
				}
			},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			err := client.WaitForNewBlock(tt.prevBlockNumber, tt.totalDuration)
			if tt.expectError {
				require.Error(t, err)
				if tt.name == "timeout waiting for new block" {
					require.Contains(t, err.Error(), "no new block produced")
				} else {
					require.Contains(t, err.Error(), "failure")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSetupProposerVM(t *testing.T) {
	originalSleepBetweenRepeats := sleepBetweenRepeats
	sleepBetweenRepeats = 1 * time.Millisecond
	defer func() {
		sleepBetweenRepeats = originalSleepBetweenRepeats
	}()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockClient := mockethclient.NewMockClient(ctrl)
	client := Client{
		EthClient: mockClient,
		URL:       "http://localhost:8545",
	}
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	privateKeyHex := hex.EncodeToString(crypto.FromECDSA(privateKey))
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	chainID := big.NewInt(43114)
	tests := []struct {
		name        string
		privateKey  string
		setupMock   func()
		expectError bool
	}{
		{
			name:       "successful setup",
			privateKey: privateKeyHex,
			setupMock: func() {
				// GetChainID
				mockClient.EXPECT().ChainID(gomock.Any()).
					Return(chainID, nil)
				// First block
				mockClient.EXPECT().BlockNumber(gomock.Any()).
					Return(uint64(1000), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), address, gomock.Any()).
					Return(uint64(0), nil)
				mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
					Return(nil)
				mockClient.EXPECT().BlockNumber(gomock.Any()).
					Return(uint64(1001), nil)
				// Second block
				mockClient.EXPECT().BlockNumber(gomock.Any()).
					Return(uint64(1001), nil)
				mockClient.EXPECT().NonceAt(gomock.Any(), address, gomock.Any()).
					Return(uint64(1), nil)
				mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
					Return(nil)
				mockClient.EXPECT().BlockNumber(gomock.Any()).
					Return(uint64(1002), nil)
			},
			expectError: false,
		},
		{
			name:       "error getting chain ID",
			privateKey: privateKeyHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure*repeatsOnFailure; i++ {
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(nil, errors.New("failed to get chain ID"))
				}
			},
			expectError: true,
		},
		{
			name:       "error getting block number",
			privateKey: privateKeyHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					// GetChainID succeeds
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(chainID, nil)
					// First block - error getting block number
					for i := 0; i < repeatsOnFailure; i++ {
						mockClient.EXPECT().BlockNumber(gomock.Any()).
							Return(uint64(0), errors.New("failed to get block number"))
					}
				}
			},
			expectError: true,
		},
		{
			name:       "error getting nonce",
			privateKey: privateKeyHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					// GetChainID succeeds
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(chainID, nil)
					// First block - error getting nonce
					mockClient.EXPECT().BlockNumber(gomock.Any()).
						Return(uint64(1000), nil)
					for i := 0; i < repeatsOnFailure; i++ {
						mockClient.EXPECT().NonceAt(gomock.Any(), address, gomock.Any()).
							Return(uint64(0), errors.New("failed to get nonce"))
					}
				}
			},
			expectError: true,
		},
		{
			name:       "error sending transaction",
			privateKey: privateKeyHex,
			setupMock: func() {
				for i := 0; i < repeatsOnFailure; i++ {
					// GetChainID succeeds
					mockClient.EXPECT().ChainID(gomock.Any()).
						Return(chainID, nil)
					// First block - error sending transaction
					mockClient.EXPECT().BlockNumber(gomock.Any()).
						Return(uint64(1000), nil)
					mockClient.EXPECT().NonceAt(gomock.Any(), address, gomock.Any()).
						Return(uint64(0), nil)
					for i := 0; i < repeatsOnFailure; i++ {
						mockClient.EXPECT().SendTransaction(gomock.Any(), gomock.Any()).
							Return(errors.New("failed to send transaction"))
					}
				}
			},
			expectError: true,
		},
		{
			name:        "invalid private key",
			privateKey:  "invalid",
			setupMock:   func() {},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			err := client.SetupProposerVM(tt.privateKey)
			if tt.expectError {
				require.Error(t, err)
				if tt.name != "invalid private key" {
					require.Contains(t, err.Error(), "failure")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
