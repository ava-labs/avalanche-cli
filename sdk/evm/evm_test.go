package evm

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	mockethclient "github.com/ava-labs/avalanche-cli/sdk/mocks/ethclient"
	"github.com/ava-labs/subnet-evm/core/types"
	subnetethclient "github.com/ava-labs/subnet-evm/ethclient"

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
				if tt.rpcURL == "http://localhost:8545" {
					require.Contains(t, err.Error(), "url does have scheme")
				} else if tt.rpcURL != "http://:invalid" {
					require.Contains(t, err.Error(), "protocol could not be determined")
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
				require.Equal(t, tt.expectedScheme, scheme)
			}
		})
	}
}

func TestGetClient(t *testing.T) {
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
				return nil, errors.New("connection error")
			},
			expectError: true,
		},
		{
			name:   "with scheme, 2 failures",
			rpcURL: "http://localhost:8545",
			mockDialFunc: func(_ context.Context, _ string) (subnetethclient.Client, error) {
				failuresCount++
				if failuresCount < 3 {
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

func TestContractAlreadyDeployed(t *testing.T) {
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
				mockClient.EXPECT().CodeAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("failed to get code"))
			},
			expected:    false,
			expectError: true,
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
				mockClient.EXPECT().BalanceAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("failed to get balance"))
			},
			expected:    nil,
			expectError: true,
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
				mockClient.EXPECT().NonceAt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(uint64(0), errors.New("failed to get nonce"))
			},
			expected:    0,
			expectError: true,
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
				mockClient.EXPECT().SuggestGasTipCap(gomock.Any()).
					Return(nil, errors.New("failed to get gas tip cap"))
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			gasTipCap, err := client.SuggestGasTipCap()
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), client.URL)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, gasTipCap)
			}
		})
	}
}

func TestEstimateBaseFee(t *testing.T) {
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
				mockClient.EXPECT().HeaderByNumber(gomock.Any(), gomock.Any()).
					Return(&types.Header{BaseFee: big.NewInt(1000000000)}, nil)
			},
			expected:    big.NewInt(1000000000),
			expectError: false,
		},
		{
			name: "error getting header",
			setupMock: func() {
				mockClient.EXPECT().HeaderByNumber(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("failed to get header"))
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			baseFee, err := client.EstimateBaseFee()
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), client.URL)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, baseFee)
			}
		})
	}
}
