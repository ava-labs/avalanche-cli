// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"bytes"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/sdk/validatormanager/validatormanagertypes"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestPromptTokenSymbol(t *testing.T) {
	tests := []struct {
		name           string
		tokenSymbol    string
		mockSetup      func(*mocks.Prompter)
		expectedResult string
		expectedError  string
	}{
		{
			name:        "returns provided token symbol when not empty",
			tokenSymbol: "AVAX",
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as CaptureString should not be called
			},
			expectedResult: "AVAX",
			expectedError:  "",
		},
		{
			name:        "returns provided token symbol when whitespace only - should not be considered empty",
			tokenSymbol: "   ",
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as CaptureString should not be called
			},
			expectedResult: "   ",
			expectedError:  "",
		},
		{
			name:        "prompts for token symbol when empty and returns captured value",
			tokenSymbol: "",
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureString", "Token Symbol").Return("MYCOIN", nil)
			},
			expectedResult: "MYCOIN",
			expectedError:  "",
		},
		{
			name:        "returns error when prompt fails",
			tokenSymbol: "",
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureString", "Token Symbol").Return("", errors.New("prompt failed"))
			},
			expectedResult: "",
			expectedError:  "prompt failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock prompter
			mockPrompter := mocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Call the function under test
			result, err := PromptTokenSymbol(app, tt.tokenSymbol)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				require.Equal(t, tt.expectedResult, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestPromptVMType(t *testing.T) {
	tests := []struct {
		name           string
		useSubnetEvm   bool
		useCustom      bool
		mockSetup      func(*mocks.Prompter)
		expectedResult models.VMType
		expectedError  string
	}{
		{
			name:         "returns SubnetEvm when useSubnetEvm is true",
			useSubnetEvm: true,
			useCustom:    false,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as CaptureList should not be called
			},
			expectedResult: models.SubnetEvm,
			expectedError:  "",
		},
		{
			name:         "returns CustomVM when useCustom is true",
			useSubnetEvm: false,
			useCustom:    true,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as CaptureList should not be called
			},
			expectedResult: models.CustomVM,
			expectedError:  "",
		},
		{
			name:         "returns SubnetEvm when useCustom is true but useSubnetEvm takes precedence",
			useSubnetEvm: true,
			useCustom:    true,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as CaptureList should not be called
			},
			expectedResult: models.SubnetEvm,
			expectedError:  "",
		},
		{
			name:         "prompts user and returns SubnetEvm when user selects Subnet-EVM option",
			useSubnetEvm: false,
			useCustom:    false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{"Subnet-EVM", "Custom VM", "Explain the difference"}
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Subnet-EVM", nil)
			},
			expectedResult: models.SubnetEvm,
			expectedError:  "",
		},
		{
			name:         "prompts user and returns CustomVM when user selects Custom VM option",
			useSubnetEvm: false,
			useCustom:    false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{"Subnet-EVM", "Custom VM", "Explain the difference"}
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Custom VM", nil)
			},
			expectedResult: models.CustomVM,
			expectedError:  "",
		},
		{
			name:         "handles explain option and then user selects Subnet-EVM",
			useSubnetEvm: false,
			useCustom:    false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{"Subnet-EVM", "Custom VM", "Explain the difference"}
				// First call returns explain option, second call returns Subnet-EVM
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Subnet-EVM", nil).Once()
			},
			expectedResult: models.SubnetEvm,
			expectedError:  "",
		},
		{
			name:         "handles explain option and then user selects Custom VM",
			useSubnetEvm: false,
			useCustom:    false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{"Subnet-EVM", "Custom VM", "Explain the difference"}
				// First call returns explain option, second call returns Custom VM
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Custom VM", nil).Once()
			},
			expectedResult: models.CustomVM,
			expectedError:  "",
		},
		{
			name:         "handles multiple explain options before user selects",
			useSubnetEvm: false,
			useCustom:    false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{"Subnet-EVM", "Custom VM", "Explain the difference"}
				// Multiple explain options, then user selects Custom VM
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("Custom VM", nil).Once()
			},
			expectedResult: models.CustomVM,
			expectedError:  "",
		},
		{
			name:         "returns error when prompt fails",
			useSubnetEvm: false,
			useCustom:    false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{"Subnet-EVM", "Custom VM", "Explain the difference"}
				m.On("CaptureList", "Which Virtual Machine would you like to use?", options).Return("", errors.New("prompt failed"))
			},
			expectedResult: "",
			expectedError:  "prompt failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock prompter
			mockPrompter := mocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Call the function under test
			result, err := PromptVMType(app, tt.useSubnetEvm, tt.useCustom)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				require.Equal(t, tt.expectedResult, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestPromptGasTokenKind(t *testing.T) {
	tests := []struct {
		name                string
		defaultsKind        DefaultsKind
		useExternalGasToken bool
		enableExternal      bool // whether to enable external gas token prompting for this test
		mockSetup           func(*mocks.Prompter)
		expectedError       string
		validateResult      func(*testing.T, SubnetEVMGenesisParams)
	}{
		{
			name:                "returns early when useExternalGasToken is true",
			defaultsKind:        NoDefaults,
			useExternalGasToken: true,
			enableExternal:      false, // doesn't matter since we return early
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as no prompting should occur
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.UseExternalGasToken)
			},
		},
		{
			name:                "no prompting when enableExternalGasTokenPrompt is false",
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			enableExternal:      false,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as no prompting should occur
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.UseExternalGasToken)
			},
		},
		{
			name:                "no prompting when defaultsKind is not NoDefaults",
			defaultsKind:        TestDefaults,
			useExternalGasToken: false,
			enableExternal:      true,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as no prompting should occur
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.UseExternalGasToken)
			},
		},
		{
			name:                "user selects native token",
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			enableExternal:      true,
			mockSetup: func(m *mocks.Prompter) {
				gasTokenOptions := []string{
					"The blockchain's native token",
					"A token from another blockchain",
					"Explain the difference",
				}
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("The blockchain's native token", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.UseExternalGasToken)
			},
		},
		{
			name:                "user selects external token",
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			enableExternal:      true,
			mockSetup: func(m *mocks.Prompter) {
				gasTokenOptions := []string{
					"The blockchain's native token",
					"A token from another blockchain",
					"Explain the difference",
				}
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("A token from another blockchain", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.UseExternalGasToken)
			},
		},
		{
			name:                "user selects explain then native token",
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			enableExternal:      true,
			mockSetup: func(m *mocks.Prompter) {
				gasTokenOptions := []string{
					"The blockchain's native token",
					"A token from another blockchain",
					"Explain the difference",
				}
				// First call returns explain option, second call returns native token
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("The blockchain's native token", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.UseExternalGasToken)
			},
		},
		{
			name:                "user selects explain then external token",
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			enableExternal:      true,
			mockSetup: func(m *mocks.Prompter) {
				gasTokenOptions := []string{
					"The blockchain's native token",
					"A token from another blockchain",
					"Explain the difference",
				}
				// First call returns explain option, second call returns external token
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("A token from another blockchain", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.UseExternalGasToken)
			},
		},
		{
			name:                "multiple explain options before selection",
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			enableExternal:      true,
			mockSetup: func(m *mocks.Prompter) {
				gasTokenOptions := []string{
					"The blockchain's native token",
					"A token from another blockchain",
					"Explain the difference",
				}
				// Multiple explain options, then user selects external token
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("A token from another blockchain", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.UseExternalGasToken)
			},
		},
		{
			name:                "prompt fails",
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			enableExternal:      true,
			mockSetup: func(m *mocks.Prompter) {
				gasTokenOptions := []string{
					"The blockchain's native token",
					"A token from another blockchain",
					"Explain the difference",
				}
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("", errors.New("prompt failed"))
			},
			expectedError: "prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up enableExternalGasTokenPrompt for this test
			originalValue := enableExternalGasTokenPrompt
			enableExternalGasTokenPrompt = tt.enableExternal
			defer func() {
				enableExternalGasTokenPrompt = originalValue
			}()

			// Create mock prompter
			mockPrompter := mocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Create initial params
			initialParams := SubnetEVMGenesisParams{}

			// Call the function under test
			result, err := promptGasTokenKind(app, tt.defaultsKind, tt.useExternalGasToken, initialParams)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				tt.validateResult(t, result)
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestPromptDefaults(t *testing.T) {
	tests := []struct {
		name           string
		inputDefaults  DefaultsKind
		mockSetup      func(*mocks.Prompter)
		expectedResult DefaultsKind
		expectedError  string
	}{
		{
			name:          "returns early when defaultsKind is TestDefaults",
			inputDefaults: TestDefaults,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as no prompting should occur
			},
			expectedResult: TestDefaults,
			expectedError:  "",
		},
		{
			name:          "returns early when defaultsKind is ProductionDefaults",
			inputDefaults: ProductionDefaults,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as no prompting should occur
			},
			expectedResult: ProductionDefaults,
			expectedError:  "",
		},
		{
			name:          "user selects test defaults",
			inputDefaults: NoDefaults,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"I want to use defaults for a test environment",
					"I want to use defaults for a production environment",
					"I don't want to use default values",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("I want to use defaults for a test environment", nil)
			},
			expectedResult: TestDefaults,
			expectedError:  "",
		},
		{
			name:          "user selects production defaults",
			inputDefaults: NoDefaults,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"I want to use defaults for a test environment",
					"I want to use defaults for a production environment",
					"I don't want to use default values",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("I want to use defaults for a production environment", nil)
			},
			expectedResult: ProductionDefaults,
			expectedError:  "",
		},
		{
			name:          "user selects no defaults (specify my values)",
			inputDefaults: NoDefaults,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"I want to use defaults for a test environment",
					"I want to use defaults for a production environment",
					"I don't want to use default values",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("I don't want to use default values", nil)
			},
			expectedResult: NoDefaults,
			expectedError:  "",
		},
		{
			name:          "user selects explain then test defaults",
			inputDefaults: NoDefaults,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"I want to use defaults for a test environment",
					"I want to use defaults for a production environment",
					"I don't want to use default values",
					"Explain the difference",
				}
				// First call returns explain option, second call returns test defaults
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("I want to use defaults for a test environment", nil).Once()
			},
			expectedResult: TestDefaults,
			expectedError:  "",
		},
		{
			name:          "user selects explain then production defaults",
			inputDefaults: NoDefaults,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"I want to use defaults for a test environment",
					"I want to use defaults for a production environment",
					"I don't want to use default values",
					"Explain the difference",
				}
				// First call returns explain option, second call returns production defaults
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("I want to use defaults for a production environment", nil).Once()
			},
			expectedResult: ProductionDefaults,
			expectedError:  "",
		},
		{
			name:          "user selects explain then no defaults",
			inputDefaults: NoDefaults,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"I want to use defaults for a test environment",
					"I want to use defaults for a production environment",
					"I don't want to use default values",
					"Explain the difference",
				}
				// First call returns explain option, second call returns no defaults
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("I don't want to use default values", nil).Once()
			},
			expectedResult: NoDefaults,
			expectedError:  "",
		},
		{
			name:          "multiple explain options before selection",
			inputDefaults: NoDefaults,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"I want to use defaults for a test environment",
					"I want to use defaults for a production environment",
					"I don't want to use default values",
					"Explain the difference",
				}
				// Multiple explain options, then user selects production defaults
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("I want to use defaults for a production environment", nil).Once()
			},
			expectedResult: ProductionDefaults,
			expectedError:  "",
		},
		{
			name:          "prompt fails",
			inputDefaults: NoDefaults,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"I want to use defaults for a test environment",
					"I want to use defaults for a production environment",
					"I don't want to use default values",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to use default values for the Blockchain configuration?", options).Return("", errors.New("prompt failed"))
			},
			expectedResult: NoDefaults,
			expectedError:  "prompt failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock prompter
			mockPrompter := mocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Call the function under test
			result, err := PromptDefaults(app, tt.inputDefaults)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				require.Equal(t, tt.expectedResult, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestDisplayAllocations(t *testing.T) {
	tests := []struct {
		name              string
		allocations       core.GenesisAlloc
		expectContains    []string
		expectNotContains []string
	}{
		{
			name: "single allocation",
			allocations: core.GenesisAlloc{
				common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"): core.GenesisAccount{
					Balance: big.NewInt(1000000000000000000), // 1 token (18 decimals)
				},
			},
			expectContains: []string{
				"ADDRESS",
				"BALANCE",
				"0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
				"1.000000000000000000", // Should be formatted as 1 token with decimals
			},
			expectNotContains: []string{},
		},
		{
			name: "multiple allocations",
			allocations: core.GenesisAlloc{
				common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"): core.GenesisAccount{
					Balance: big.NewInt(1000000000000000000), // 1 token
				},
				common.HexToAddress("0x1111111111111111111111111111111111111111"): core.GenesisAccount{
					Balance: big.NewInt(2000000000000000000), // 2 tokens
				},
			},
			expectContains: []string{
				"ADDRESS",
				"BALANCE",
				"0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC",
				"0x1111111111111111111111111111111111111111",
				"1.000000000000000000",
				"2.000000000000000000",
			},
			expectNotContains: []string{},
		},
		{
			name: "large balance formatting",
			allocations: core.GenesisAlloc{
				common.HexToAddress("0x2222222222222222222222222222222222222222"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(1000000), big.NewInt(1000000000000000000)), // 1 million tokens
				},
			},
			expectContains: []string{
				"ADDRESS",
				"BALANCE",
				"0x2222222222222222222222222222222222222222",
				"1000000.000000000000000000", // Should be formatted as 1,000,000 tokens with decimals
			},
			expectNotContains: []string{},
		},
		{
			name: "fractional balance",
			allocations: core.GenesisAlloc{
				common.HexToAddress("0x3333333333333333333333333333333333333333"): core.GenesisAccount{
					Balance: big.NewInt(500000000000000000), // 0.5 tokens
				},
			},
			expectContains: []string{
				"ADDRESS",
				"BALANCE",
				"0x3333333333333333333333333333333333333333",
				"0.500000000000000000", // Should be formatted as 0.5 tokens with decimals
			},
			expectNotContains: []string{},
		},
		{
			name:              "empty allocations",
			allocations:       core.GenesisAlloc{},
			expectContains:    []string{"ADDRESS", "BALANCE"}, // Headers should still appear
			expectNotContains: []string{"0x"},                 // No addresses should appear
		},
		{
			name: "zero balance",
			allocations: core.GenesisAlloc{
				common.HexToAddress("0x4444444444444444444444444444444444444444"): core.GenesisAccount{
					Balance: big.NewInt(0),
				},
			},
			expectContains: []string{
				"ADDRESS",
				"BALANCE",
				"0x4444444444444444444444444444444444444444",
				"0.000000000000000000", // Should show 0 with decimals
			},
			expectNotContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output using a buffer
			var buf bytes.Buffer

			// Call the function with our buffer
			displayAllocations(tt.allocations, &buf)

			// Get the output
			output := buf.String()

			// Verify expected content is present
			for _, expected := range tt.expectContains {
				require.Contains(t, output, expected, "Output should contain: %s", expected)
			}

			// Verify unwanted content is not present
			for _, notExpected := range tt.expectNotContains {
				require.NotContains(t, output, notExpected, "Output should not contain: %s", notExpected)
			}

			// Verify that output is not empty (unless allocations are empty)
			if len(tt.allocations) > 0 {
				require.NotEmpty(t, output, "Output should not be empty for non-empty allocations")
			}
		})
	}
}

func TestAddNewKeyAllocation(t *testing.T) {
	tests := []struct {
		name           string
		subnetName     string
		setupDir       func(string) // Function to set up the temp directory
		expectedError  string
		validateResult func(*testing.T, core.GenesisAlloc, string) // Added tempDir parameter
	}{
		{
			name:       "successful key allocation",
			subnetName: "test-subnet",
			setupDir: func(tempDir string) {
				// Create the key directory
				keyDir := filepath.Join(tempDir, constants.KeyDir)
				err := os.MkdirAll(keyDir, 0755)
				require.NoError(t, err)
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc, tempDir string) {
				// Should have exactly one allocation
				require.Len(t, allocations, 1)

				// Check that allocation was added with the correct balance
				found := false
				for addr, account := range allocations {
					require.NotEmpty(t, addr.Hex())
					require.Equal(t, defaultEVMAirdropAmount, account.Balance)
					found = true
				}
				require.True(t, found, "Expected allocation to be added")

				// Verify that a key file was actually created
				app := &application.Avalanche{}
				app.Setup(tempDir, logging.NoLog{}, &config.Config{}, "test-version", nil, nil, nil)
				expectedKeyName := utils.GetDefaultBlockchainAirdropKeyName("test-subnet")
				keyPath := app.GetKeyPath(expectedKeyName)
				require.FileExists(t, keyPath)
			},
		},
		{
			name:       "successful allocation with different blockchain name",
			subnetName: "another-test-blockchain",
			setupDir: func(tempDir string) {
				// Create the key directory
				keyDir := filepath.Join(tempDir, constants.KeyDir)
				err := os.MkdirAll(keyDir, 0755)
				require.NoError(t, err)
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc, tempDir string) {
				// Should have exactly one allocation
				require.Len(t, allocations, 1)

				// Check that allocation was added with the correct balance
				found := false
				for addr, account := range allocations {
					require.NotEmpty(t, addr.Hex())
					require.Equal(t, defaultEVMAirdropAmount, account.Balance)
					found = true
				}
				require.True(t, found, "Expected allocation to be added")

				// Verify that a key file was actually created
				app := &application.Avalanche{}
				app.Setup(tempDir, logging.NoLog{}, &config.Config{}, "test-version", nil, nil, nil)
				expectedKeyName := utils.GetDefaultBlockchainAirdropKeyName("another-test-blockchain")
				keyPath := app.GetKeyPath(expectedKeyName)
				require.FileExists(t, keyPath)
			},
		},
		{
			name:       "validates key name generation",
			subnetName: "special-blockchain-name",
			setupDir: func(tempDir string) {
				// Create the key directory
				keyDir := filepath.Join(tempDir, constants.KeyDir)
				err := os.MkdirAll(keyDir, 0755)
				require.NoError(t, err)
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc, tempDir string) {
				require.Len(t, allocations, 1)

				// Verify the key was created with the expected name pattern
				app := &application.Avalanche{}
				app.Setup(tempDir, logging.NoLog{}, &config.Config{}, "test-version", nil, nil, nil)
				expectedKeyName := utils.GetDefaultBlockchainAirdropKeyName("special-blockchain-name")
				keyPath := app.GetKeyPath(expectedKeyName)
				require.FileExists(t, keyPath)

				// Key name should contain blockchain name
				require.Contains(t, expectedKeyName, "special-blockchain-name")
			},
		},
		{
			name:       "GetKey fails when key file cannot be created",
			subnetName: "error-blockchain",
			setupDir: func(tempDir string) {
				// Create the key directory first
				keyDir := filepath.Join(tempDir, constants.KeyDir)
				err := os.MkdirAll(keyDir, 0755)
				require.NoError(t, err)

				// Get the expected key name and create a directory where the key file should be
				expectedKeyName := utils.GetDefaultBlockchainAirdropKeyName("error-blockchain")
				keyFilePath := filepath.Join(keyDir, expectedKeyName+constants.KeySuffix)
				// Create a directory instead of allowing a file to be created
				err = os.MkdirAll(keyFilePath, 0755)
				require.NoError(t, err)
			},
			expectedError: "is a directory",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc, tempDir string) {
				// Should have no allocations since the function failed
				require.Len(t, allocations, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for the test
			tempDir := t.TempDir()

			// Set up the directory as needed for the test
			tt.setupDir(tempDir)

			// Create a real application instance
			app := &application.Avalanche{}
			app.Setup(
				tempDir,
				logging.NoLog{},
				&config.Config{},
				"test-version",
				nil, // We don't need prompter for this test
				nil, // We don't need downloader for this test
				nil, // We don't need cmd for this test
			)

			// Create initial empty allocations
			allocations := make(core.GenesisAlloc)

			// Call the function under test
			err := addNewKeyAllocation(allocations, app, tt.subnetName)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}

			// Validate the result
			tt.validateResult(t, allocations, tempDir)
		})
	}
}

func TestAddEwoqAllocation(t *testing.T) {
	tests := []struct {
		name               string
		initialAllocations core.GenesisAlloc
		validateResult     func(*testing.T, core.GenesisAlloc)
	}{
		{
			name:               "adds ewoq allocation to empty allocations",
			initialAllocations: make(core.GenesisAlloc),
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should have exactly one allocation
				require.Len(t, allocations, 1)

				// Check that the ewoq address was added with correct balance
				account, exists := allocations[PrefundedEwoqAddress]
				require.True(t, exists, "Ewoq address should exist in allocations")
				require.Equal(t, defaultEVMAirdropAmount, account.Balance, "Balance should be defaultEVMAirdropAmount")
				require.NotNil(t, account.Balance, "Balance should not be nil")
			},
		},
		{
			name: "adds ewoq allocation to existing allocations",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x1111111111111111111111111111111111111111"): core.GenesisAccount{
					Balance: big.NewInt(5000000000000000000), // 5 tokens
				},
				common.HexToAddress("0x2222222222222222222222222222222222222222"): core.GenesisAccount{
					Balance: big.NewInt(3000000000000000000), // 3 tokens
				},
			},
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should have three allocations now (2 existing + 1 ewoq)
				require.Len(t, allocations, 3)

				// Check that the ewoq address was added with correct balance
				account, exists := allocations[PrefundedEwoqAddress]
				require.True(t, exists, "Ewoq address should exist in allocations")
				require.Equal(t, defaultEVMAirdropAmount, account.Balance, "Balance should be defaultEVMAirdropAmount")

				// Check that existing allocations were not modified
				existingAccount1, exists1 := allocations[common.HexToAddress("0x1111111111111111111111111111111111111111")]
				require.True(t, exists1, "Existing address 1 should still exist")
				require.Equal(t, big.NewInt(5000000000000000000), existingAccount1.Balance, "Existing balance 1 should be unchanged")

				existingAccount2, exists2 := allocations[common.HexToAddress("0x2222222222222222222222222222222222222222")]
				require.True(t, exists2, "Existing address 2 should still exist")
				require.Equal(t, big.NewInt(3000000000000000000), existingAccount2.Balance, "Existing balance 2 should be unchanged")
			},
		},
		{
			name: "overwrites existing ewoq allocation",
			initialAllocations: core.GenesisAlloc{
				PrefundedEwoqAddress: core.GenesisAccount{
					Balance: big.NewInt(1000000000000000000), // 1 token - different from default
				},
				common.HexToAddress("0x3333333333333333333333333333333333333333"): core.GenesisAccount{
					Balance: big.NewInt(2000000000000000000), // 2 tokens
				},
			},
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should still have two allocations
				require.Len(t, allocations, 2)

				// Check that the ewoq address balance was overwritten with the default amount
				account, exists := allocations[PrefundedEwoqAddress]
				require.True(t, exists, "Ewoq address should exist in allocations")
				require.Equal(t, defaultEVMAirdropAmount, account.Balance, "Balance should be defaultEVMAirdropAmount (overwritten)")
				require.NotEqual(t, big.NewInt(1000000000000000000), account.Balance, "Balance should not be the old value")

				// Check that other allocations were not modified
				existingAccount, exists := allocations[common.HexToAddress("0x3333333333333333333333333333333333333333")]
				require.True(t, exists, "Existing address should still exist")
				require.Equal(t, big.NewInt(2000000000000000000), existingAccount.Balance, "Existing balance should be unchanged")
			},
		},
		{
			name: "idempotent - multiple calls result in same state",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x4444444444444444444444444444444444444444"): core.GenesisAccount{
					Balance: big.NewInt(7000000000000000000), // 7 tokens
				},
			},
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Call the function multiple times
				addEwoqAllocation(allocations)
				addEwoqAllocation(allocations)
				addEwoqAllocation(allocations)

				// Should have two allocations (1 existing + 1 ewoq)
				require.Len(t, allocations, 2)

				// Check that the ewoq address has the correct balance
				account, exists := allocations[PrefundedEwoqAddress]
				require.True(t, exists, "Ewoq address should exist in allocations")
				require.Equal(t, defaultEVMAirdropAmount, account.Balance, "Balance should be defaultEVMAirdropAmount")

				// Check that existing allocation is unchanged
				existingAccount, exists := allocations[common.HexToAddress("0x4444444444444444444444444444444444444444")]
				require.True(t, exists, "Existing address should still exist")
				require.Equal(t, big.NewInt(7000000000000000000), existingAccount.Balance, "Existing balance should be unchanged")
			},
		},
		{
			name:               "validates ewoq address constant",
			initialAllocations: make(core.GenesisAlloc),
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Verify the exact ewoq address that should be used
				expectedAddress := common.HexToAddress("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC")
				require.Equal(t, expectedAddress, PrefundedEwoqAddress, "PrefundedEwoqAddress should be the expected ewoq address")

				// Check that this exact address was added
				account, exists := allocations[expectedAddress]
				require.True(t, exists, "Expected ewoq address should exist in allocations")
				require.Equal(t, defaultEVMAirdropAmount, account.Balance, "Balance should be defaultEVMAirdropAmount")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of initial allocations to avoid test interference
			allocations := make(core.GenesisAlloc)
			for addr, account := range tt.initialAllocations {
				allocations[addr] = account
			}

			// Call the function under test
			addEwoqAllocation(allocations)

			// Validate the result
			tt.validateResult(t, allocations)
		})
	}
}

func TestGetNativeGasTokenAllocationConfig(t *testing.T) {
	tests := []struct {
		name               string
		initialAllocations core.GenesisAlloc
		subnetName         string
		tokenSymbol        string
		mockSetup          func(*mocks.Prompter)
		expectedError      string
		validateResult     func(*testing.T, core.GenesisAlloc)
	}{
		{
			name:               "allocate to new key option",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", options).Return("Allocate 1m tokens to a new account", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 1)

				// Should have exactly one allocation with the default amount
				found := false
				for addr, account := range allocations {
					require.NotEmpty(t, addr.Hex())
					require.Equal(t, defaultEVMAirdropAmount, account.Balance)
					found = true
				}
				require.True(t, found, "Expected allocation to be added")
			},
		},
		{
			name:               "allocate to ewoq option",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", options).Return("Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 1)

				// Verify the ewoq allocation
				account, exists := allocations[PrefundedEwoqAddress]
				require.True(t, exists)
				require.Equal(t, defaultEVMAirdropAmount, account.Balance)
			},
		},
		{
			name:               "custom allocation - add address and confirm",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - add address
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Add an address to the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to allocate to").Return(common.HexToAddress("0x2222222222222222222222222222222222222222"), nil).Once()
				m.On("CaptureUint64", "Amount to allocate (in TEST units)").Return(uint64(5), nil).Once()

				// Second action - confirm
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(true, nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 1)

				// Verify the custom allocation (5 TEST tokens = 5 * 1e18)
				account, exists := allocations[common.HexToAddress("0x2222222222222222222222222222222222222222")]
				require.True(t, exists)
				expectedBalance := new(big.Int).Mul(big.NewInt(5), OneAvax)
				require.Equal(t, expectedBalance, account.Balance)
			},
		},
		{
			name: "custom allocation - change existing address",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x3333333333333333333333333333333333333333"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(10), OneAvax),
				},
			},
			subnetName:  "testSubnet",
			tokenSymbol: "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - change address
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Edit the amount of an address in the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to update the allocation of").Return(common.HexToAddress("0x3333333333333333333333333333333333333333"), nil).Once()
				m.On("CaptureUint64", "Updated amount to allocate (in TEST units)").Return(uint64(15), nil).Once()

				// Second action - confirm
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(true, nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 1)

				// Verify the updated allocation (15 TEST tokens = 15 * 1e18)
				account, exists := allocations[common.HexToAddress("0x3333333333333333333333333333333333333333")]
				require.True(t, exists)
				expectedBalance := new(big.Int).Mul(big.NewInt(15), OneAvax)
				require.Equal(t, expectedBalance, account.Balance)
			},
		},
		{
			name: "custom allocation - remove address and confirm",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x4444444444444444444444444444444444444444"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(20), OneAvax),
				},
				common.HexToAddress("0x5555555555555555555555555555555555555555"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(30), OneAvax),
				},
			},
			subnetName:  "testSubnet",
			tokenSymbol: "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - remove address
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Remove an address from the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to remove from the allocation list").Return(common.HexToAddress("0x4444444444444444444444444444444444444444"), nil).Once()

				// Second action - confirm
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(true, nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 1)

				// Verify the address was removed and only the second address remains
				_, exists1 := allocations[common.HexToAddress("0x4444444444444444444444444444444444444444")]
				require.False(t, exists1, "First address should be removed")

				account2, exists2 := allocations[common.HexToAddress("0x5555555555555555555555555555555555555555")]
				require.True(t, exists2, "Second address should still exist")
				expectedBalance := new(big.Int).Mul(big.NewInt(30), OneAvax)
				require.Equal(t, expectedBalance, account2.Balance)
			},
		},
		{
			name:               "custom allocation - confirm without changes says no then yes",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - try to confirm
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(false, nil).Once()

				// Second action - try again and say yes
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(true, nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 0)
			},
		},
		{
			name:               "initial prompt fails",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", options).Return("", errors.New("prompt failed"))
			},
			expectedError: "prompt failed",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should not be called due to error
			},
		},
		{
			name:               "custom allocation - add address but capture address fails",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - add address but fails
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Add an address to the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to allocate to").Return(common.Address{}, errors.New("address capture failed")).Once()
			},
			expectedError: "address capture failed",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should not be called due to error
			},
		},
		{
			name:               "custom allocation - add address but capture balance fails",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - add address, address succeeds but balance fails
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Add an address to the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to allocate to").Return(common.HexToAddress("0x2222222222222222222222222222222222222222"), nil).Once()
				m.On("CaptureUint64", "Amount to allocate (in TEST units)").Return(uint64(0), errors.New("balance capture failed")).Once()
			},
			expectedError: "balance capture failed",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should not be called due to error
			},
		},
		{
			name:               "custom allocation - action selection prompt fails",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type succeeds
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// Second prompt - action selection fails
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("", errors.New("action selection failed")).Once()
			},
			expectedError: "action selection failed",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should not be called due to error
			},
		},
		{
			name: "custom allocation - try to add address that already exists",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x1111111111111111111111111111111111111111"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(100), OneAvax),
				},
			},
			subnetName:  "testSubnet",
			tokenSymbol: "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - try to add existing address (should print message and continue)
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Add an address to the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to allocate to").Return(common.HexToAddress("0x1111111111111111111111111111111111111111"), nil).Once()

				// Second action - confirm (since the add was skipped due to existing address)
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(true, nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 1)

				// Verify the original allocation remains unchanged
				account, exists := allocations[common.HexToAddress("0x1111111111111111111111111111111111111111")]
				require.True(t, exists)
				expectedBalance := new(big.Int).Mul(big.NewInt(100), OneAvax)
				require.Equal(t, expectedBalance, account.Balance)
			},
		},
		{
			name: "custom allocation - try to change address that doesn't exist",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x2222222222222222222222222222222222222222"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(50), OneAvax),
				},
			},
			subnetName:  "testSubnet",
			tokenSymbol: "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - try to change non-existing address (should print message and continue)
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Edit the amount of an address in the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to update the allocation of").Return(common.HexToAddress("0x9999999999999999999999999999999999999999"), nil).Once()

				// Second action - confirm (since the change was skipped due to non-existing address)
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(true, nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 1)

				// Verify the original allocation remains unchanged and the non-existing address wasn't added
				account, exists := allocations[common.HexToAddress("0x2222222222222222222222222222222222222222")]
				require.True(t, exists)
				expectedBalance := new(big.Int).Mul(big.NewInt(50), OneAvax)
				require.Equal(t, expectedBalance, account.Balance)

				// Verify the non-existing address wasn't added
				_, exists = allocations[common.HexToAddress("0x9999999999999999999999999999999999999999")]
				require.False(t, exists)
			},
		},
		{
			name: "custom allocation - try to remove address that doesn't exist",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x3333333333333333333333333333333333333333"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(75), OneAvax),
				},
			},
			subnetName:  "testSubnet",
			tokenSymbol: "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - try to remove non-existing address (should print message and continue)
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Remove an address from the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to remove from the allocation list").Return(common.HexToAddress("0x8888888888888888888888888888888888888888"), nil).Once()

				// Second action - confirm (since the remove was skipped due to non-existing address)
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(true, nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 1)

				// Verify the original allocation remains unchanged
				account, exists := allocations[common.HexToAddress("0x3333333333333333333333333333333333333333")]
				require.True(t, exists)
				expectedBalance := new(big.Int).Mul(big.NewInt(75), OneAvax)
				require.Equal(t, expectedBalance, account.Balance)

				// Verify the non-existing address is still not there (of course)
				_, exists = allocations[common.HexToAddress("0x8888888888888888888888888888888888888888")]
				require.False(t, exists)
			},
		},
		{
			name:               "custom allocation - change address but capture address fails",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - change address but address capture fails
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Edit the amount of an address in the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to update the allocation of").Return(common.Address{}, errors.New("change address capture failed")).Once()
			},
			expectedError: "change address capture failed",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should not be called due to error
			},
		},
		{
			name:               "custom allocation - remove address but capture address fails",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - remove address but address capture fails
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Remove an address from the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to remove from the allocation list").Return(common.Address{}, errors.New("remove address capture failed")).Once()
			},
			expectedError: "remove address capture failed",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should not be called due to error
			},
		},
		{
			name: "custom allocation - change address but capture balance fails",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x4444444444444444444444444444444444444444"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(25), OneAvax),
				},
			},
			subnetName:  "testSubnet",
			tokenSymbol: "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - change address, address succeeds but balance fails
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Edit the amount of an address in the initial token allocation", nil).Once()
				m.On("CaptureAddress", "Address to update the allocation of").Return(common.HexToAddress("0x4444444444444444444444444444444444444444"), nil).Once()
				m.On("CaptureUint64", "Updated amount to allocate (in TEST units)").Return(uint64(0), errors.New("change balance capture failed")).Once()
			},
			expectedError: "change balance capture failed",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should not be called due to error
			},
		},
		{
			name: "custom allocation - preview allocation (displayAllocations executes)",
			initialAllocations: core.GenesisAlloc{
				common.HexToAddress("0x5555555555555555555555555555555555555555"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(100), OneAvax),
				},
				common.HexToAddress("0x6666666666666666666666666666666666666666"): core.GenesisAccount{
					Balance: new(big.Int).Mul(big.NewInt(200), OneAvax),
				},
			},
			subnetName:  "testSubnet",
			tokenSymbol: "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - preview allocation (this will call displayAllocations)
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Preview the initial token allocation", nil).Once()

				// Second action - confirm
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(true, nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				require.Len(t, allocations, 2)

				// Verify both allocations remain unchanged
				account1, exists1 := allocations[common.HexToAddress("0x5555555555555555555555555555555555555555")]
				require.True(t, exists1)
				expectedBalance1 := new(big.Int).Mul(big.NewInt(100), OneAvax)
				require.Equal(t, expectedBalance1, account1.Balance)

				account2, exists2 := allocations[common.HexToAddress("0x6666666666666666666666666666666666666666")]
				require.True(t, exists2)
				expectedBalance2 := new(big.Int).Mul(big.NewInt(200), OneAvax)
				require.Equal(t, expectedBalance2, account2.Balance)
			},
		},
		{
			name:               "custom allocation - confirm but CaptureYesNo fails",
			initialAllocations: make(core.GenesisAlloc),
			subnetName:         "testSubnet",
			tokenSymbol:        "TEST",
			mockSetup: func(m *mocks.Prompter) {
				allocationOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				actionOptions := []string{
					"Add an address to the initial token allocation",
					"Edit the amount of an address in the initial token allocation",
					"Remove an address from the initial token allocation",
					"Preview the initial token allocation",
					"Confirm and finalize the initial token allocation",
				}

				// First prompt - allocation type
				m.On("CaptureList", "How should the initial token allocation be structured?", allocationOptions).Return("Define a custom allocation (Recommended for production)", nil).Once()

				// First action - confirm but CaptureYesNo fails
				m.On("CaptureList", "How would you like to modify the initial token allocation?", actionOptions).Return("Confirm and finalize the initial token allocation", nil).Once()
				m.On("CaptureYesNo", "Are you sure you want to finalize this allocation list?").Return(false, errors.New("yes/no capture failed")).Once()
			},
			expectedError: "yes/no capture failed",
			validateResult: func(t *testing.T, allocations core.GenesisAlloc) {
				// Should not be called due to error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var app *application.Avalanche
			var tempDir string
			var mockPrompter *mocks.Prompter

			// Special handling for "allocate to new key option" test which requires real file operations
			if tt.name == "allocate to new key option" {
				// Create a temporary directory for the test
				tempDir = t.TempDir()

				// Create the key directory
				keyDir := filepath.Join(tempDir, constants.KeyDir)
				err := os.MkdirAll(keyDir, 0755)
				require.NoError(t, err)

				// Create a real application instance
				app = &application.Avalanche{}
				app.Setup(
					tempDir,
					logging.NoLog{},
					&config.Config{},
					"test-version",
					nil, // We don't need prompter for key creation
					nil, // We don't need downloader for this test
					nil, // We don't need cmd for this test
				)

				// Create mock prompter for the initial selection
				mockPrompter = mocks.NewPrompter(t)
				tt.mockSetup(mockPrompter)
				app.Prompt = mockPrompter
			} else {
				// For other tests, use mock-only approach
				// Create mock prompter
				mockPrompter = mocks.NewPrompter(t)

				// Set up mock expectations
				tt.mockSetup(mockPrompter)

				// Create application with mock prompter
				app = &application.Avalanche{
					Prompt: mockPrompter,
				}
			}

			// Create a copy of initial allocations to avoid test interference
			allocations := make(core.GenesisAlloc)
			for addr, account := range tt.initialAllocations {
				allocations[addr] = account
			}

			// Call the function under test
			err := getNativeGasTokenAllocationConfig(allocations, app, tt.subnetName, tt.tokenSymbol)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				tt.validateResult(t, allocations)

				// Additional validation for "allocate to new key option" test
				if tt.name == "allocate to new key option" {
					// Verify that a key file was actually created
					expectedKeyName := utils.GetDefaultBlockchainAirdropKeyName(tt.subnetName)
					keyPath := app.GetKeyPath(expectedKeyName)
					require.FileExists(t, keyPath)
				}
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestPromptSubnetEVMGenesisParams(t *testing.T) {
	// Helper function to create test app
	createTestApp := func(mockPrompter *mocks.Prompter) *application.Avalanche {
		return &application.Avalanche{
			Prompt: mockPrompter,
		}
	}

	tests := []struct {
		name                string
		sidecar             *models.Sidecar
		version             string
		chainID             uint64
		tokenSymbol         string
		blockchainName      string
		useICM              *bool
		defaultsKind        DefaultsKind
		useWarp             bool
		useExternalGasToken bool
		mockSetup           func(*mocks.Prompter)
		expectedError       string
		validateResult      func(*testing.T, SubnetEVMGenesisParams, string)
	}{
		{
			name: "test defaults with basic sidecar",
			sidecar: &models.Sidecar{
				Name: "test-blockchain",
			},
			version:             "v0.6.8",
			chainID:             12345,
			tokenSymbol:         "TEST",
			blockchainName:      "test-blockchain",
			useICM:              nil,
			defaultsKind:        TestDefaults,
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// With TestDefaults, most prompting should be skipped
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.Equal(t, uint64(12345), params.chainID)
				require.Equal(t, "TEST", tokenSymbol)
				require.True(t, params.UseICM)
				require.True(t, params.enableWarpPrecompile)
				require.False(t, params.UseExternalGasToken)
				require.True(t, params.feeConfig.lowThroughput)
				require.False(t, params.feeConfig.useDynamicFees)
				// Should have ewoq allocation
				require.Len(t, params.initialTokenAllocation, 1)
			},
		},
		// Skip production defaults test as it requires key generation
		{
			name: "sidecar with PoA should set PoA params",
			sidecar: &models.Sidecar{
				Name:                  "poa-blockchain",
				ValidatorManagement:   validatormanagertypes.ProofOfAuthority,
				ValidatorManagerOwner: "0x1234567890123456789012345678901234567890",
			},
			version:             "v0.6.8",
			chainID:             98765,
			tokenSymbol:         "POA",
			blockchainName:      "poa-blockchain",
			useICM:              nil,
			defaultsKind:        TestDefaults,
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// With TestDefaults, most prompting should be skipped
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.True(t, params.UsePoAValidatorManager)
				require.False(t, params.UsePoSValidatorManager)
				require.True(t, params.DisableICMOnGenesis)
				// Should have both PoA owner allocation and ewoq allocation
				require.Len(t, params.initialTokenAllocation, 2)
				// Check that the PoA owner has the right balance
				poaOwnerAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
				account, exists := params.initialTokenAllocation[poaOwnerAddr]
				require.True(t, exists)
				require.Equal(t, defaultPoAOwnerBalance, account.Balance)
			},
		},
		{
			name: "sidecar with PoS should set PoS params",
			sidecar: &models.Sidecar{
				Name:                "pos-blockchain",
				ValidatorManagement: validatormanagertypes.ProofOfStake,
			},
			version:             "v0.6.8",
			chainID:             11111,
			tokenSymbol:         "POS",
			blockchainName:      "pos-blockchain",
			useICM:              nil,
			defaultsKind:        TestDefaults,
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// With TestDefaults, most prompting should be skipped
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.False(t, params.UsePoAValidatorManager)
				require.True(t, params.UsePoSValidatorManager)
				require.True(t, params.enableNativeMinterPrecompile)
				require.True(t, params.enableRewardManagerPrecompile)
				require.True(t, params.DisableICMOnGenesis)
				// Should have ewoq allocation
				require.Len(t, params.initialTokenAllocation, 1)
			},
		},
		{
			name: "chainID prompting when chainID is 0",
			sidecar: &models.Sidecar{
				Name: "prompt-chainid-blockchain",
			},
			version:             "v0.6.8",
			chainID:             0, // This should trigger prompting
			tokenSymbol:         "PROMPT",
			blockchainName:      "prompt-chainid-blockchain",
			useICM:              nil,
			defaultsKind:        TestDefaults,
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureUint64", "Chain ID").Return(uint64(77777), nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.Equal(t, uint64(77777), params.chainID)
			},
		},
		{
			name: "error when chainID prompting fails",
			sidecar: &models.Sidecar{
				Name: "error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             0, // This should trigger prompting
			tokenSymbol:         "ERROR",
			blockchainName:      "error-blockchain",
			useICM:              nil,
			defaultsKind:        TestDefaults,
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureUint64", "Chain ID").Return(uint64(0), errors.New("chainID prompt failed"))
			},
			expectedError: "chainID prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name: "warp validation error when ICM is enabled but warp is disabled",
			sidecar: &models.Sidecar{
				Name: "warp-error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             99999,
			tokenSymbol:         "WARP",
			blockchainName:      "warp-error-blockchain",
			useICM:              func() *bool { b := true; return &b }(), // Enable ICM
			defaultsKind:        TestDefaults,
			useWarp:             false, // But disable warp - this should cause an error
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// With TestDefaults, most prompting should be skipped
			},
			expectedError: "warp should be enabled for ICM to work",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name: "external gas token should enable ICM automatically",
			sidecar: &models.Sidecar{
				Name: "external-gas-blockchain",
			},
			version:             "v0.6.8",
			chainID:             88888,
			tokenSymbol:         "EXT",
			blockchainName:      "external-gas-blockchain",
			useICM:              nil,
			defaultsKind:        TestDefaults,
			useWarp:             true,
			useExternalGasToken: true, // This should automatically enable ICM
			mockSetup: func(m *mocks.Prompter) {
				// With TestDefaults and external gas token, most prompting should be skipped
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.True(t, params.UseExternalGasToken)
				require.True(t, params.UseICM) // Should be enabled automatically
				require.True(t, params.enableWarpPrecompile)
				// Should not have any token allocations since using external gas token
				require.Len(t, params.initialTokenAllocation, 0)
			},
		},
		{
			name: "error when promptGasTokenKind fails",
			sidecar: &models.Sidecar{
				Name: "gas-token-kind-error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             99999,
			tokenSymbol:         "GASTKN",
			blockchainName:      "gas-token-kind-error-blockchain",
			useICM:              nil,
			defaultsKind:        NoDefaults, // This triggers prompting when enableExternalGasTokenPrompt is true
			useWarp:             true,
			useExternalGasToken: false, // This ensures promptGasTokenKind is called
			mockSetup: func(m *mocks.Prompter) {
				// promptGasTokenKind will prompt since enableExternalGasTokenPrompt is enabled for this test
				gasTokenOptions := []string{
					"The blockchain's native token",
					"A token from another blockchain",
					"Explain the difference",
				}
				m.On("CaptureList", "Which token will be used for transaction fee payments?", gasTokenOptions).Return("", errors.New("gas token kind prompt failed"))
			},
			expectedError: "gas token kind prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name: "error when promptFeeConfig fails - throughput prompt",
			sidecar: &models.Sidecar{
				Name: "fee-config-error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             11111,
			tokenSymbol:         "ERR",
			blockchainName:      "fee-config-error-blockchain",
			useICM:              nil,
			defaultsKind:        NoDefaults, // This will trigger prompting
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// First, handle the allocation prompt to get past promptNativeGasToken
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)", nil)

				// Then handle native minter precompile prompt
				nativeMinterOptions := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", nativeMinterOptions).Return("No, I want the supply of the native tokens be hard-capped", nil)

				// Now promptFeeConfig will fail on the first prompt (throughput selection)
				feeOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", feeOptions).Return("", errors.New("fee config prompt failed"))
			},
			expectedError: "fee config prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name: "error when promptNativeGasToken fails - token symbol prompt",
			sidecar: &models.Sidecar{
				Name: "native-gas-error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             22222,
			tokenSymbol:         "", // Empty to trigger prompting
			blockchainName:      "native-gas-error-blockchain",
			useICM:              nil,
			defaultsKind:        NoDefaults, // This will trigger prompting
			useWarp:             true,
			useExternalGasToken: false, // This ensures promptNativeGasToken is called
			mockSetup: func(m *mocks.Prompter) {
				// promptNativeGasToken calls PromptTokenSymbol first
				m.On("CaptureString", "Token Symbol").Return("", errors.New("token symbol prompt failed"))
			},
			expectedError: "token symbol prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name: "error when promptNativeGasToken fails - allocation prompt",
			sidecar: &models.Sidecar{
				Name: "allocation-error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             33333,
			tokenSymbol:         "ALLOC",
			blockchainName:      "allocation-error-blockchain",
			useICM:              nil,
			defaultsKind:        NoDefaults, // This will trigger prompting
			useWarp:             true,
			useExternalGasToken: false, // This ensures promptNativeGasToken is called
			mockSetup: func(m *mocks.Prompter) {
				// promptNativeGasToken won't prompt for token symbol since it's provided
				// But it will prompt for allocation type
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("", errors.New("allocation prompt failed"))
			},
			expectedError: "allocation prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name: "error when promptFeeConfig fails - dynamic fees prompt",
			sidecar: &models.Sidecar{
				Name: "fee-config-dynamic-error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             44444,
			tokenSymbol:         "FEE",
			blockchainName:      "fee-config-dynamic-error-blockchain",
			useICM:              nil,
			defaultsKind:        NoDefaults, // This will trigger prompting
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// First, handle the allocation prompt to get past promptNativeGasToken
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)", nil)

				// Then handle native minter precompile prompt
				nativeMinterOptions := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", nativeMinterOptions).Return("No, I want the supply of the native tokens be hard-capped", nil)

				// Now handle fee config - first prompt (throughput selection)
				feeOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", feeOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				// Then the dynamic fees prompt should fail
				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("", errors.New("dynamic fees prompt failed"))
			},
			expectedError: "dynamic fees prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name: "error when PromptInterop fails",
			sidecar: &models.Sidecar{
				Name: "interop-error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             55555,
			tokenSymbol:         "INTEROP",
			blockchainName:      "interop-error-blockchain",
			useICM:              nil,        // This will trigger prompting
			defaultsKind:        NoDefaults, // This will trigger prompting
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// First, handle the allocation prompt to get past promptNativeGasToken
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)", nil)

				// Handle native minter precompile prompt
				nativeMinterOptions := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", nativeMinterOptions).Return("No, I want the supply of the native tokens be hard-capped", nil)

				// Handle fee config prompts
				feeOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", feeOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				adjustFeeOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", adjustFeeOptions).Return("No, use the transaction fee configuration set in the genesis block", nil)

				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("Yes, I want the transaction fees to be burned", nil)

				// Now the PromptInterop should fail
				interopOptions := []string{
					"Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain",
					"No, I want to run my blockchain isolated",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to connect your blockchain with other blockchains or the C-Chain?", interopOptions).Return("", errors.New("interop prompt failed"))
			},
			expectedError: "interop prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name: "error when promptPermissioning fails",
			sidecar: &models.Sidecar{
				Name: "permissioning-error-blockchain",
			},
			version:             "v0.6.8",
			chainID:             66666,
			tokenSymbol:         "PERM",
			blockchainName:      "permissioning-error-blockchain",
			useICM:              func() *bool { b := true; return &b }(), // Provide ICM to skip that prompt
			defaultsKind:        NoDefaults,                              // This will trigger prompting
			useWarp:             true,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// Handle all the earlier prompts successfully
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)", nil)

				nativeMinterOptions := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", nativeMinterOptions).Return("No, I want the supply of the native tokens be hard-capped", nil)

				feeOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", feeOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				adjustFeeOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", adjustFeeOptions).Return("No, use the transaction fee configuration set in the genesis block", nil)

				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("Yes, I want the transaction fees to be burned", nil)

				// Now the permissioning prompt should fail
				permissioningOptions := []string{
					"Yes",
					"No",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", permissioningOptions).Return("", errors.New("permissioning prompt failed"))
			},
			expectedError: "permissioning prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Special handling for promptGasTokenKind test - enable external gas token prompting
			if tt.name == "error when promptGasTokenKind fails" {
				originalValue := enableExternalGasTokenPrompt
				enableExternalGasTokenPrompt = true
				defer func() {
					enableExternalGasTokenPrompt = originalValue
				}()
			}

			// Create mock prompter
			mockPrompter := mocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := createTestApp(mockPrompter)

			// Call the function under test
			result, tokenSymbol, err := PromptSubnetEVMGenesisParams(
				app,
				tt.sidecar,
				tt.version,
				tt.chainID,
				tt.tokenSymbol,
				tt.blockchainName,
				tt.useICM,
				tt.defaultsKind,
				tt.useWarp,
				tt.useExternalGasToken,
			)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				tt.validateResult(t, result, tokenSymbol)
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestGetNativeMinterPrecompileConfig(t *testing.T) {
	tests := []struct {
		name             string
		alreadyEnabled   bool
		initialAllowList AllowList
		version          string
		mockSetup        func(*mocks.Prompter)
		expectedError    string
		validateResult   func(*testing.T, AllowList, bool)
	}{
		{
			name:             "not already enabled - user selects fixed supply",
			alreadyEnabled:   false,
			initialAllowList: AllowList{},
			version:          "v0.6.8",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", options).Return("No, I want the supply of the native tokens be hard-capped", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, allowList AllowList, enabled bool) {
				require.False(t, enabled)
				require.Empty(t, allowList.AdminAddresses)
				require.Empty(t, allowList.ManagerAddresses)
				require.Empty(t, allowList.EnabledAddresses)
			},
		},
		{
			name:             "not already enabled - user selects dynamic supply and configures allow list",
			alreadyEnabled:   false,
			initialAllowList: AllowList{},
			version:          "v0.6.8",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", options).Return("Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)", nil)

				// Mock GenerateAllowList flow - add admin address and confirm
				allowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to mint native tokens", allowListOptions).Return("Add an address for a role to the allow list", nil).Once()

				roleOptions := []string{
					"Admin",
					"Manager",
					"Enabled",
					"Explain the difference",
					"Cancel",
				}
				m.On("CaptureList", "What role should the address have?", roleOptions).Return("Admin", nil).Once()
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return([]common.Address{common.HexToAddress("0x1111111111111111111111111111111111111111")}, nil).Once()

				// Confirm the allow list
				allowListOptionsWithRemove := []string{
					"Add an address for a role to the allow list",
					"Remove address from the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to mint native tokens", allowListOptionsWithRemove).Return("Confirm Allow List", nil).Once()

				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allowList AllowList, enabled bool) {
				require.True(t, enabled)
				require.Len(t, allowList.AdminAddresses, 1)
				require.Equal(t, common.HexToAddress("0x1111111111111111111111111111111111111111"), allowList.AdminAddresses[0])
				require.Empty(t, allowList.ManagerAddresses)
				require.Empty(t, allowList.EnabledAddresses)
			},
		},
		{
			name:             "not already enabled - CaptureList fails",
			alreadyEnabled:   false,
			initialAllowList: AllowList{},
			version:          "v0.6.8",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", options).Return("", errors.New("capture list failed"))
			},
			expectedError: "capture list failed",
			validateResult: func(t *testing.T, allowList AllowList, enabled bool) {
				// Should not be called due to error
			},
		},
		{
			name:           "already enabled - user chooses not to configure allow list",
			alreadyEnabled: true,
			initialAllowList: AllowList{
				AdminAddresses: []common.Address{common.HexToAddress("0x2222222222222222222222222222222222222222")},
			},
			version: "v0.6.8",
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureYesNo", "Minting of native tokens automatically enabled. Do you want to configure allow list?").Return(false, nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, allowList AllowList, enabled bool) {
				require.False(t, enabled)
				require.Empty(t, allowList.AdminAddresses)
				require.Empty(t, allowList.ManagerAddresses)
				require.Empty(t, allowList.EnabledAddresses)
			},
		},
		{
			name:           "already enabled - user chooses to configure allow list",
			alreadyEnabled: true,
			initialAllowList: AllowList{
				AdminAddresses: []common.Address{common.HexToAddress("0x3333333333333333333333333333333333333333")},
			},
			version: "v0.6.8",
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureYesNo", "Minting of native tokens automatically enabled. Do you want to configure allow list?").Return(true, nil)

				// Mock GenerateAllowList flow - just confirm existing
				allowListOptions := []string{
					"Add an address for a role to the allow list",
					"Remove address from the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to mint native tokens", allowListOptions).Return("Confirm Allow List", nil).Once()

				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allowList AllowList, enabled bool) {
				require.True(t, enabled)
				require.Len(t, allowList.AdminAddresses, 1)
				require.Equal(t, common.HexToAddress("0x3333333333333333333333333333333333333333"), allowList.AdminAddresses[0])
				require.Empty(t, allowList.ManagerAddresses)
				require.Empty(t, allowList.EnabledAddresses)
			},
		},
		{
			name:             "already enabled - CaptureYesNo fails",
			alreadyEnabled:   true,
			initialAllowList: AllowList{},
			version:          "v0.6.8",
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureYesNo", "Minting of native tokens automatically enabled. Do you want to configure allow list?").Return(false, errors.New("yes no capture failed"))
			},
			expectedError: "yes no capture failed",
			validateResult: func(t *testing.T, allowList AllowList, enabled bool) {
				// Should not be called due to error
			},
		},
		{
			name:             "dynamic supply - GenerateAllowList fails",
			alreadyEnabled:   false,
			initialAllowList: AllowList{},
			version:          "v0.6.8",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", options).Return("Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)", nil)

				// Mock GenerateAllowList failure
				allowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to mint native tokens", allowListOptions).Return("", errors.New("generate allow list failed"))
			},
			expectedError: "generate allow list failed",
			validateResult: func(t *testing.T, allowList AllowList, enabled bool) {
				// Should not be called due to error
			},
		},
		{
			name:             "dynamic supply - GenerateAllowList cancelled and retried",
			alreadyEnabled:   false,
			initialAllowList: AllowList{},
			version:          "v0.6.8",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", options).Return("Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)", nil)

				// Mock GenerateAllowList cancelled first, then successful
				allowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to mint native tokens", allowListOptions).Return("Cancel", nil).Once()

				// Second attempt - successful
				m.On("CaptureList", "Configure the addresses that are allowed to mint native tokens", allowListOptions).Return("Confirm Allow List", nil).Once()
				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, allowList AllowList, enabled bool) {
				require.True(t, enabled)
				require.Empty(t, allowList.AdminAddresses)
				require.Empty(t, allowList.ManagerAddresses)
				require.Empty(t, allowList.EnabledAddresses)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock prompter
			mockPrompter := mocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Call the function under test
			resultAllowList, enabled, err := getNativeMinterPrecompileConfig(
				app,
				tt.alreadyEnabled,
				tt.initialAllowList,
				tt.version,
			)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				tt.validateResult(t, resultAllowList, enabled)
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestPromptNativeGasToken(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		tokenSymbol    string
		blockchainName string
		defaultsKind   DefaultsKind
		initialParams  SubnetEVMGenesisParams
		mockSetup      func(*mocks.Prompter)
		expectedError  string
		validateResult func(*testing.T, SubnetEVMGenesisParams, string)
	}{
		{
			name:           "test defaults - token symbol provided",
			version:        "v0.6.8",
			tokenSymbol:    "TEST",
			blockchainName: "test-blockchain",
			defaultsKind:   TestDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation: make(core.GenesisAlloc),
			},
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as token symbol is provided and TestDefaults doesn't prompt
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.Equal(t, "TEST", tokenSymbol)
				require.Len(t, params.initialTokenAllocation, 1)
				// Verify ewoq allocation
				account, exists := params.initialTokenAllocation[PrefundedEwoqAddress]
				require.True(t, exists)
				require.Equal(t, defaultEVMAirdropAmount, account.Balance)
			},
		},
		{
			name:           "test defaults - token symbol empty and prompted",
			version:        "v0.6.8",
			tokenSymbol:    "",
			blockchainName: "test-blockchain",
			defaultsKind:   TestDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation: make(core.GenesisAlloc),
			},
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureString", "Token Symbol").Return("PROMPTED", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.Equal(t, "PROMPTED", tokenSymbol)
				require.Len(t, params.initialTokenAllocation, 1)
				// Verify ewoq allocation
				account, exists := params.initialTokenAllocation[PrefundedEwoqAddress]
				require.True(t, exists)
				require.Equal(t, defaultEVMAirdropAmount, account.Balance)
			},
		},
		{
			name:           "production defaults - token symbol provided",
			version:        "v0.6.8",
			tokenSymbol:    "PROD",
			blockchainName: "prod-blockchain",
			defaultsKind:   ProductionDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation: make(core.GenesisAlloc),
			},
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as token symbol is provided and ProductionDefaults only creates a key
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.Equal(t, "PROD", tokenSymbol)
				require.Len(t, params.initialTokenAllocation, 1)
				// Verify new key allocation exists (we can't predict the exact address)
				found := false
				for addr, account := range params.initialTokenAllocation {
					if addr != PrefundedEwoqAddress && account.Balance.Cmp(defaultEVMAirdropAmount) == 0 {
						found = true
						break
					}
				}
				require.True(t, found, "Expected new key allocation to be created")
			},
		},
		{
			name:           "no defaults - complete flow with allocation and minter",
			version:        "v0.6.8",
			tokenSymbol:    "CUSTOM",
			blockchainName: "custom-blockchain",
			defaultsKind:   NoDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation:          make(core.GenesisAlloc),
				enableNativeMinterPrecompile:    false,
				nativeMinterPrecompileAllowList: AllowList{},
			},
			mockSetup: func(m *mocks.Prompter) {
				// Mock getNativeGasTokenAllocationConfig
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)", nil)

				// Mock getNativeMinterPrecompileConfig
				nativeMinterOptions := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", nativeMinterOptions).Return("Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)", nil)

				// Mock GenerateAllowList for native minter
				allowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to mint native tokens", allowListOptions).Return("Add an address for a role to the allow list", nil).Once()

				roleOptions := []string{
					"Admin",
					"Manager",
					"Enabled",
					"Explain the difference",
					"Cancel",
				}
				m.On("CaptureList", "What role should the address have?", roleOptions).Return("Admin", nil).Once()
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return([]common.Address{common.HexToAddress("0x1111111111111111111111111111111111111111")}, nil).Once()

				allowListOptionsWithRemove := []string{
					"Add an address for a role to the allow list",
					"Remove address from the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to mint native tokens", allowListOptionsWithRemove).Return("Confirm Allow List", nil).Once()

				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.Equal(t, "CUSTOM", tokenSymbol)
				require.Len(t, params.initialTokenAllocation, 1)
				// Verify ewoq allocation from getNativeGasTokenAllocationConfig
				account, exists := params.initialTokenAllocation[PrefundedEwoqAddress]
				require.True(t, exists)
				require.Equal(t, defaultEVMAirdropAmount, account.Balance)
				// Verify native minter is enabled
				require.True(t, params.enableNativeMinterPrecompile)
				require.Len(t, params.nativeMinterPrecompileAllowList.AdminAddresses, 1)
				require.Equal(t, common.HexToAddress("0x1111111111111111111111111111111111111111"), params.nativeMinterPrecompileAllowList.AdminAddresses[0])
			},
		},
		{
			name:           "token symbol prompt fails",
			version:        "v0.6.8",
			tokenSymbol:    "",
			blockchainName: "error-blockchain",
			defaultsKind:   TestDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation: make(core.GenesisAlloc),
			},
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureString", "Token Symbol").Return("", errors.New("token symbol prompt failed"))
			},
			expectedError: "token symbol prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name:           "production defaults - addNewKeyAllocation fails",
			version:        "v0.6.8",
			tokenSymbol:    "PRODERR",
			blockchainName: "", // Empty blockchain name should cause addNewKeyAllocation to fail
			defaultsKind:   ProductionDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation: make(core.GenesisAlloc),
			},
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as the error will come from key generation
			},
			expectedError: "no such file or directory",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name:           "no defaults - getNativeGasTokenAllocationConfig fails",
			version:        "v0.6.8",
			tokenSymbol:    "ALLOCERR",
			blockchainName: "alloc-error-blockchain",
			defaultsKind:   NoDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation: make(core.GenesisAlloc),
			},
			mockSetup: func(m *mocks.Prompter) {
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("", errors.New("allocation config failed"))
			},
			expectedError: "allocation config failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name:           "no defaults - getNativeMinterPrecompileConfig fails",
			version:        "v0.6.8",
			tokenSymbol:    "MINTERERR",
			blockchainName: "minter-error-blockchain",
			defaultsKind:   NoDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation:          make(core.GenesisAlloc),
				enableNativeMinterPrecompile:    false,
				nativeMinterPrecompileAllowList: AllowList{},
			},
			mockSetup: func(m *mocks.Prompter) {
				// Mock successful getNativeGasTokenAllocationConfig
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)", nil)

				// Mock failing getNativeMinterPrecompileConfig
				nativeMinterOptions := []string{
					"No, I want the supply of the native tokens be hard-capped",
					"Yes, I want to be able to mint additional the native tokens (Native Minter Precompile ON)",
				}
				m.On("CaptureList", "Allow minting of new native tokens?", nativeMinterOptions).Return("", errors.New("minter config failed"))
			},
			expectedError: "minter config failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				// Should not be called due to error
			},
		},
		{
			name:           "no defaults - minter already enabled",
			version:        "v0.6.8",
			tokenSymbol:    "ENABLED",
			blockchainName: "enabled-blockchain",
			defaultsKind:   NoDefaults,
			initialParams: SubnetEVMGenesisParams{
				initialTokenAllocation:       make(core.GenesisAlloc),
				enableNativeMinterPrecompile: true,
				nativeMinterPrecompileAllowList: AllowList{
					AdminAddresses: []common.Address{common.HexToAddress("0x2222222222222222222222222222222222222222")},
				},
			},
			mockSetup: func(m *mocks.Prompter) {
				// Mock getNativeGasTokenAllocationConfig
				allocOptions := []string{
					"Allocate 1m tokens to a new account",
					"Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)",
					"Define a custom allocation (Recommended for production)",
				}
				m.On("CaptureList", "How should the initial token allocation be structured?", allocOptions).Return("Allocate 1m to the ewoq account 0x8db...2FC (Only recommended for testing, not recommended for production)", nil)

				// Mock getNativeMinterPrecompileConfig with already enabled
				m.On("CaptureYesNo", "Minting of native tokens automatically enabled. Do you want to configure allow list?").Return(false, nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams, tokenSymbol string) {
				require.Equal(t, "ENABLED", tokenSymbol)
				require.Len(t, params.initialTokenAllocation, 1)
				// Verify ewoq allocation from getNativeGasTokenAllocationConfig
				account, exists := params.initialTokenAllocation[PrefundedEwoqAddress]
				require.True(t, exists)
				require.Equal(t, defaultEVMAirdropAmount, account.Balance)
				// Verify native minter is disabled (user chose not to configure)
				require.False(t, params.enableNativeMinterPrecompile)
				require.Empty(t, params.nativeMinterPrecompileAllowList.AdminAddresses)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var app *application.Avalanche
			var tempDir string

			// Special handling for production defaults test which requires real file operations
			if tt.defaultsKind == ProductionDefaults && tt.expectedError == "" {
				// Create a temporary directory for the test
				tempDir = t.TempDir()

				// Create the key directory
				keyDir := filepath.Join(tempDir, constants.KeyDir)
				err := os.MkdirAll(keyDir, 0755)
				require.NoError(t, err)

				// Create a real application instance
				app = &application.Avalanche{}
				app.Setup(
					tempDir,
					logging.NoLog{},
					&config.Config{},
					"test-version",
					nil, // We don't need prompter for key creation
					nil, // We don't need downloader for this test
					nil, // We don't need cmd for this test
				)

				// Create mock prompter for any prompting needs
				mockPrompter := mocks.NewPrompter(t)
				tt.mockSetup(mockPrompter)
				app.Prompt = mockPrompter
			} else {
				// For other tests, use mock-only approach
				// Create mock prompter
				mockPrompter := mocks.NewPrompter(t)

				// Set up mock expectations
				tt.mockSetup(mockPrompter)

				// Create application with mock prompter
				app = &application.Avalanche{
					Prompt: mockPrompter,
				}
			}

			// Call the function under test
			resultParams, resultTokenSymbol, err := promptNativeGasToken(
				app,
				tt.version,
				tt.tokenSymbol,
				tt.blockchainName,
				tt.defaultsKind,
				tt.initialParams,
			)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				tt.validateResult(t, resultParams, resultTokenSymbol)

				// Additional validation for production defaults test
				if tt.defaultsKind == ProductionDefaults && tt.expectedError == "" {
					// Verify that a key file was actually created
					expectedKeyName := utils.GetDefaultBlockchainAirdropKeyName(tt.blockchainName)
					keyPath := app.GetKeyPath(expectedKeyName)
					require.FileExists(t, keyPath)
				}
			}

			// Verify all mock expectations were met
			app.Prompt.(*mocks.Prompter).AssertExpectations(t)
		})
	}
}

func TestPromptFeeConfig(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		defaultsKind   DefaultsKind
		initialParams  SubnetEVMGenesisParams
		mockSetup      func(*mocks.Prompter)
		expectedError  string
		validateResult func(*testing.T, SubnetEVMGenesisParams)
	}{
		{
			name:          "test defaults",
			version:       "v0.6.8",
			defaultsKind:  TestDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as TestDefaults doesn't prompt
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.lowThroughput)
				require.False(t, params.feeConfig.useDynamicFees)
				require.False(t, params.enableFeeManagerPrecompile)
				require.False(t, params.enableRewardManagerPrecompile)
			},
		},
		{
			name:          "production defaults",
			version:       "v0.6.8",
			defaultsKind:  ProductionDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as ProductionDefaults doesn't prompt
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.lowThroughput)
				require.False(t, params.feeConfig.useDynamicFees)
				require.False(t, params.enableFeeManagerPrecompile)
				require.False(t, params.enableRewardManagerPrecompile)
			},
		},
		{
			name:          "no defaults - low throughput, no dynamic fees, no fee manager, burn fees",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				// Throughput selection
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				// Dynamic fees selection
				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				// Fee adjustability selection
				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("No, use the transaction fee configuration set in the genesis block", nil)

				// Fee burning selection
				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("Yes, I want the transaction fees to be burned", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.lowThroughput)
				require.False(t, params.feeConfig.useDynamicFees)
				require.False(t, params.enableFeeManagerPrecompile)
				require.False(t, params.enableRewardManagerPrecompile)
			},
		},
		{
			name:          "no defaults - medium throughput, dynamic fees, fee manager, reward manager",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				// Throughput selection
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)", nil)

				// Dynamic fees selection
				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("Yes, I would like my blockchain to have dynamic fees", nil)

				// Fee adjustability selection
				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)", nil)

				// Mock GenerateAllowList for fee manager
				feeManagerOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to adjust the gas fees", feeManagerOptions).Return("Confirm Allow List", nil).Once()
				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil).Once()

				// Fee burning selection
				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)", nil)

				// Mock GenerateAllowList for reward manager
				rewardManagerOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to customize gas fees distribution", rewardManagerOptions).Return("Confirm Allow List", nil).Once()
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.mediumThroughput)
				require.True(t, params.feeConfig.useDynamicFees)
				require.True(t, params.enableFeeManagerPrecompile)
				require.True(t, params.enableRewardManagerPrecompile)
			},
		},
		{
			name:          "no defaults - high throughput",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				// Throughput selection
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("High block size   / High Throughput   20 mil gas per block", nil)

				// Dynamic fees selection
				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				// Fee adjustability selection
				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("No, use the transaction fee configuration set in the genesis block", nil)

				// Fee burning selection
				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("Yes, I want the transaction fees to be burned", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.highThroughput)
				require.False(t, params.feeConfig.useDynamicFees)
				require.False(t, params.enableFeeManagerPrecompile)
				require.False(t, params.enableRewardManagerPrecompile)
			},
		},
		{
			name:          "no defaults - custom fee config",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				// Throughput selection
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)

				// Custom fee config prompts
				m.On("CapturePositiveBigInt", "Set gas limit").Return(big.NewInt(8000000), nil)
				m.On("CapturePositiveBigInt", "Set target block rate").Return(big.NewInt(2), nil)
				m.On("CapturePositiveBigInt", "Set min base fee").Return(big.NewInt(25000000000), nil)
				m.On("CapturePositiveBigInt", "Set target gas").Return(big.NewInt(15000000), nil)
				m.On("CapturePositiveBigInt", "Set base fee change denominator").Return(big.NewInt(36), nil)
				m.On("CapturePositiveBigInt", "Set min block gas cost").Return(big.NewInt(0), nil)
				m.On("CapturePositiveBigInt", "Set max block gas cost").Return(big.NewInt(1000000), nil)
				m.On("CapturePositiveBigInt", "Set block gas cost step").Return(big.NewInt(200000), nil)

				// Dynamic fees selection
				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("Yes, I would like my blockchain to have dynamic fees", nil)

				// Fee adjustability selection
				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("No, use the transaction fee configuration set in the genesis block", nil)

				// Fee burning selection
				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("Yes, I want the transaction fees to be burned", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.useDynamicFees)
				require.Equal(t, big.NewInt(8000000), params.feeConfig.gasLimit)
				require.Equal(t, big.NewInt(2), params.feeConfig.blockRate)
				require.Equal(t, big.NewInt(25000000000), params.feeConfig.minBaseFee)
				require.Equal(t, big.NewInt(15000000), params.feeConfig.targetGas)
				require.Equal(t, big.NewInt(36), params.feeConfig.baseDenominator)
				require.Equal(t, big.NewInt(0), params.feeConfig.minBlockGas)
				require.Equal(t, big.NewInt(1000000), params.feeConfig.maxBlockGas)
				require.Equal(t, big.NewInt(200000), params.feeConfig.gasStep)
				require.False(t, params.enableFeeManagerPrecompile)
				require.False(t, params.enableRewardManagerPrecompile)
			},
		},
		{
			name:          "no defaults - explain then select option",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				// Throughput selection with explain
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil).Once()

				// Dynamic fees selection with explain
				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil).Once()

				// Fee adjustability selection with explain
				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("No, use the transaction fee configuration set in the genesis block", nil).Once()

				// Fee burning selection with explain
				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("Explain the difference", nil).Once()
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("Yes, I want the transaction fees to be burned", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.lowThroughput)
				require.False(t, params.feeConfig.useDynamicFees)
				require.False(t, params.enableFeeManagerPrecompile)
				require.False(t, params.enableRewardManagerPrecompile)
			},
		},
		{
			name:          "throughput prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("", errors.New("throughput prompt failed"))
			},
			expectedError: "throughput prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - gas limit prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(nil, errors.New("gas limit prompt failed"))
			},
			expectedError: "gas limit prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "dynamic fees prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("", errors.New("dynamic fees prompt failed"))
			},
			expectedError: "dynamic fees prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "fee adjustability prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("", errors.New("fee adjust prompt failed"))
			},
			expectedError: "fee adjust prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "fee manager GenerateAllowList fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)", nil)

				// Mock GenerateAllowList failure
				feeManagerOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to adjust the gas fees", feeManagerOptions).Return("", errors.New("fee manager allow list failed"))
			},
			expectedError: "fee manager allow list failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "fee burning prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("No, use the transaction fee configuration set in the genesis block", nil)

				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("", errors.New("burn fees prompt failed"))
			},
			expectedError: "burn fees prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "reward manager GenerateAllowList fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("No, use the transaction fee configuration set in the genesis block", nil)

				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)", nil)

				// Mock GenerateAllowList failure
				rewardManagerOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to customize gas fees distribution", rewardManagerOptions).Return("", errors.New("reward manager allow list failed"))
			},
			expectedError: "reward manager allow list failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - gas limit prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(nil, errors.New("gas limit prompt failed"))
			},
			expectedError: "gas limit prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - block rate prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(big.NewInt(8000000), nil)
				m.On("CapturePositiveBigInt", "Set target block rate").Return(nil, errors.New("block rate prompt failed"))
			},
			expectedError: "block rate prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - min base fee prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(big.NewInt(8000000), nil)
				m.On("CapturePositiveBigInt", "Set target block rate").Return(big.NewInt(2), nil)
				m.On("CapturePositiveBigInt", "Set min base fee").Return(nil, errors.New("min base fee prompt failed"))
			},
			expectedError: "min base fee prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - target gas prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(big.NewInt(8000000), nil)
				m.On("CapturePositiveBigInt", "Set target block rate").Return(big.NewInt(2), nil)
				m.On("CapturePositiveBigInt", "Set min base fee").Return(big.NewInt(25000000000), nil)
				m.On("CapturePositiveBigInt", "Set target gas").Return(nil, errors.New("target gas prompt failed"))
			},
			expectedError: "target gas prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - base fee change denominator prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(big.NewInt(8000000), nil)
				m.On("CapturePositiveBigInt", "Set target block rate").Return(big.NewInt(2), nil)
				m.On("CapturePositiveBigInt", "Set min base fee").Return(big.NewInt(25000000000), nil)
				m.On("CapturePositiveBigInt", "Set target gas").Return(big.NewInt(15000000), nil)
				m.On("CapturePositiveBigInt", "Set base fee change denominator").Return(nil, errors.New("base fee change denominator prompt failed"))
			},
			expectedError: "base fee change denominator prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - min block gas prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(big.NewInt(8000000), nil)
				m.On("CapturePositiveBigInt", "Set target block rate").Return(big.NewInt(2), nil)
				m.On("CapturePositiveBigInt", "Set min base fee").Return(big.NewInt(25000000000), nil)
				m.On("CapturePositiveBigInt", "Set target gas").Return(big.NewInt(15000000), nil)
				m.On("CapturePositiveBigInt", "Set base fee change denominator").Return(big.NewInt(36), nil)
				m.On("CapturePositiveBigInt", "Set min block gas cost").Return(nil, errors.New("min block gas prompt failed"))
			},
			expectedError: "min block gas prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - max block gas prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(big.NewInt(8000000), nil)
				m.On("CapturePositiveBigInt", "Set target block rate").Return(big.NewInt(2), nil)
				m.On("CapturePositiveBigInt", "Set min base fee").Return(big.NewInt(25000000000), nil)
				m.On("CapturePositiveBigInt", "Set target gas").Return(big.NewInt(15000000), nil)
				m.On("CapturePositiveBigInt", "Set base fee change denominator").Return(big.NewInt(36), nil)
				m.On("CapturePositiveBigInt", "Set min block gas cost").Return(big.NewInt(0), nil)
				m.On("CapturePositiveBigInt", "Set max block gas cost").Return(nil, errors.New("max block gas prompt failed"))
			},
			expectedError: "max block gas prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "custom fee config - gas step prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Customize fee config", nil)
				m.On("CapturePositiveBigInt", "Set gas limit").Return(big.NewInt(8000000), nil)
				m.On("CapturePositiveBigInt", "Set target block rate").Return(big.NewInt(2), nil)
				m.On("CapturePositiveBigInt", "Set min base fee").Return(big.NewInt(25000000000), nil)
				m.On("CapturePositiveBigInt", "Set target gas").Return(big.NewInt(15000000), nil)
				m.On("CapturePositiveBigInt", "Set base fee change denominator").Return(big.NewInt(36), nil)
				m.On("CapturePositiveBigInt", "Set min block gas cost").Return(big.NewInt(0), nil)
				m.On("CapturePositiveBigInt", "Set max block gas cost").Return(big.NewInt(1000000), nil)
				m.On("CapturePositiveBigInt", "Set block gas cost step").Return(nil, errors.New("gas step prompt failed"))
			},
			expectedError: "gas step prompt failed",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "fee manager GenerateAllowList cancelled and retried",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				// First time: user selects to enable fee manager
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)", nil).Once()

				// Mock GenerateAllowList - first call cancelled
				feeManagerOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to adjust the gas fees", feeManagerOptions).Return("Cancel", nil).Once()

				// Second time: user decides not to enable fee manager
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("No, use the transaction fee configuration set in the genesis block", nil).Once()

				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("Yes, I want the transaction fees to be burned", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.lowThroughput)
				require.False(t, params.feeConfig.useDynamicFees)
				require.False(t, params.enableFeeManagerPrecompile) // Should be false since user cancelled and then chose No
				require.False(t, params.enableRewardManagerPrecompile)
			},
		},
		{
			name:          "reward manager GenerateAllowList cancelled and retried",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *mocks.Prompter) {
				throughputOptions := []string{
					"Low block size    / Low Throughput    12 mil gas per block",
					"Medium block size / Medium Throughput 15 mil gas per block (C-Chain's setting)",
					"High block size   / High Throughput   20 mil gas per block",
					"Customize fee config",
					"Explain the difference",
				}
				m.On("CaptureList", "How should the transaction fees be configured on your Blockchain?", throughputOptions).Return("Low block size    / Low Throughput    12 mil gas per block", nil)

				dynamicFeeOptions := []string{
					"No, I prefer to have constant gas prices",
					"Yes, I would like my blockchain to have dynamic fees",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want dynamic fees on your blockchain?", dynamicFeeOptions).Return("No, I prefer to have constant gas prices", nil)

				feeAdjustOptions := []string{
					"No, use the transaction fee configuration set in the genesis block",
					"Yes, allow adjustment of the transaction fee configuration as needed. Recommended for production (Fee Manager Precompile ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Should transaction fees be adjustable without a network upgrade?", feeAdjustOptions).Return("No, use the transaction fee configuration set in the genesis block", nil)

				burnFeeOptions := []string{
					"Yes, I want the transaction fees to be burned",
					"No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)",
					"Explain the difference",
				}
				// First time: user selects to enable reward manager
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)", nil).Once()

				// Mock GenerateAllowList - first call cancelled
				rewardManagerOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to customize gas fees distribution", rewardManagerOptions).Return("Cancel", nil).Once()

				// Second time: user selects to enable reward manager again
				m.On("CaptureList", "Do you want the transaction fees to be burned (sent to a blackhole address)? All transaction fees on Avalanche are burned by default", burnFeeOptions).Return("No, I want to customize accumulated transaction fees distribution (Reward Manager Precompile ON)", nil).Once()

				// Mock GenerateAllowList - second call succeeds
				m.On("CaptureList", "Configure the addresses that are allowed to customize gas fees distribution", rewardManagerOptions).Return("Confirm Allow List", nil).Once()
				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.feeConfig.lowThroughput)
				require.False(t, params.feeConfig.useDynamicFees)
				require.False(t, params.enableFeeManagerPrecompile)
				require.True(t, params.enableRewardManagerPrecompile) // Should be true since user succeeded on retry
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock prompter
			mockPrompter := mocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Call the function under test
			resultParams, err := promptFeeConfig(
				app,
				tt.version,
				tt.defaultsKind,
				tt.initialParams,
			)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				tt.validateResult(t, resultParams)
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestPromptInterop(t *testing.T) {
	tests := []struct {
		name                string
		useICMFlag          *bool
		defaultsKind        DefaultsKind
		useExternalGasToken bool
		mockSetup           func(*mocks.Prompter)
		expectedResult      bool
		expectedError       string
	}{
		{
			name:                "useICMFlag is true",
			useICMFlag:          func() *bool { b := true; return &b }(),
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as function returns early
			},
			expectedResult: true,
			expectedError:  "",
		},
		{
			name:                "useICMFlag is false",
			useICMFlag:          func() *bool { b := false; return &b }(),
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as function returns early
			},
			expectedResult: false,
			expectedError:  "",
		},
		{
			name:                "defaultsKind is TestDefaults",
			useICMFlag:          nil,
			defaultsKind:        TestDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as function returns early
			},
			expectedResult: true,
			expectedError:  "",
		},
		{
			name:                "defaultsKind is ProductionDefaults",
			useICMFlag:          nil,
			defaultsKind:        ProductionDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as function returns early
			},
			expectedResult: true,
			expectedError:  "",
		},
		{
			name:                "useExternalGasToken is true",
			useICMFlag:          nil,
			defaultsKind:        NoDefaults,
			useExternalGasToken: true,
			mockSetup: func(m *mocks.Prompter) {
				// No mock setup needed as function returns early
			},
			expectedResult: true,
			expectedError:  "",
		},
		{
			name:                "user selects interoperating blockchain",
			useICMFlag:          nil,
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain",
					"No, I want to run my blockchain isolated",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to connect your blockchain with other blockchains or the C-Chain?", options).Return("Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain", nil)
			},
			expectedResult: true,
			expectedError:  "",
		},
		{
			name:                "user selects isolated blockchain",
			useICMFlag:          nil,
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain",
					"No, I want to run my blockchain isolated",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to connect your blockchain with other blockchains or the C-Chain?", options).Return("No, I want to run my blockchain isolated", nil)
			},
			expectedResult: false,
			expectedError:  "",
		},
		{
			name:                "user selects explain then interoperating",
			useICMFlag:          nil,
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain",
					"No, I want to run my blockchain isolated",
					"Explain the difference",
				}
				// First call: user selects explain
				m.On("CaptureList", "Do you want to connect your blockchain with other blockchains or the C-Chain?", options).Return("Explain the difference", nil).Once()
				// Second call: user selects interoperating
				m.On("CaptureList", "Do you want to connect your blockchain with other blockchains or the C-Chain?", options).Return("Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain", nil).Once()
			},
			expectedResult: true,
			expectedError:  "",
		},
		{
			name:                "user selects explain then isolated",
			useICMFlag:          nil,
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain",
					"No, I want to run my blockchain isolated",
					"Explain the difference",
				}
				// First call: user selects explain
				m.On("CaptureList", "Do you want to connect your blockchain with other blockchains or the C-Chain?", options).Return("Explain the difference", nil).Once()
				// Second call: user selects isolated
				m.On("CaptureList", "Do you want to connect your blockchain with other blockchains or the C-Chain?", options).Return("No, I want to run my blockchain isolated", nil).Once()
			},
			expectedResult: false,
			expectedError:  "",
		},
		{
			name:                "CaptureList fails",
			useICMFlag:          nil,
			defaultsKind:        NoDefaults,
			useExternalGasToken: false,
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"Yes, I want to enable my blockchain to interoperate with other blockchains and the C-Chain",
					"No, I want to run my blockchain isolated",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to connect your blockchain with other blockchains or the C-Chain?", options).Return("", errors.New("capture list failed"))
			},
			expectedResult: false,
			expectedError:  "capture list failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock prompter
			mockPrompter := mocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Call the function under test
			result, err := PromptInterop(
				app,
				tt.useICMFlag,
				tt.defaultsKind,
				tt.useExternalGasToken,
			)

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}
