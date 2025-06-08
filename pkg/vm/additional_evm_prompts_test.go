// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"errors"
	"os"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	promptMocks "github.com/ava-labs/avalanche-cli/pkg/prompts/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testSubnetEVMVersion074 = `{"subnet-evm": "v0.7.4"}`
	testSubnetEVMVersion073 = `{"subnet-evm": "v0.7.3"}`
)

func TestPromptPermissioning(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		defaultsKind   DefaultsKind
		initialParams  SubnetEVMGenesisParams
		mockSetup      func(*promptMocks.Prompter)
		expectedError  string
		validateResult func(*testing.T, SubnetEVMGenesisParams)
	}{
		{
			name:          "test defaults - no prompting",
			version:       "v0.6.8",
			defaultsKind:  TestDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(_ *promptMocks.Prompter) {
				// No mock setup needed as function returns early
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile)
				require.False(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "production defaults - no prompting",
			version:       "v0.6.8",
			defaultsKind:  ProductionDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(_ *promptMocks.Prompter) {
				// No mock setup needed as function returns early
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile)
				require.False(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "user enables anyone for everything",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				options := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", options).Return("Yes", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile)
				require.False(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "user restricts both transactions and contract deployment",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				// Transaction permissions
				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)", nil)

				// Mock GenerateAllowList for transactions
				transactionAllowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to issue transactions", transactionAllowListOptions).Return("Confirm Allow List", nil)
				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil)

				// Contract deployment permissions
				contractOptions := []string{
					"Yes, I want anyone to be able to deploy smart contracts on my blockchain",
					"No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)", nil)

				// Mock GenerateAllowList for contract deployment
				contractAllowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to deploy smart contracts", contractAllowListOptions).Return("Confirm Allow List", nil)
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.True(t, params.enableTransactionPrecompile)
				require.True(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "user allows anyone for transactions but restricts contract deployment",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				// Transaction permissions - allow anyone
				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("Yes, I want anyone to be able to issue transactions on my blockchain", nil)

				// Contract deployment permissions - restrict
				contractOptions := []string{
					"Yes, I want anyone to be able to deploy smart contracts on my blockchain",
					"No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)", nil)

				// Mock GenerateAllowList for contract deployment
				contractAllowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to deploy smart contracts", contractAllowListOptions).Return("Confirm Allow List", nil)
				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile)
				require.True(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "user explains main option then allows everything",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				options := []string{"Yes", "No", "Explain the difference"}
				// First call: user selects explain
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", options).Return("Explain the difference", nil).Once()
				// Second call: user selects yes
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", options).Return("Yes", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile)
				require.False(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "user explains transaction option then allows anyone",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				// Transaction permissions with explain
				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				// First call: user selects explain
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("Explain the difference", nil).Once()
				// Second call: user selects allow anyone
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("Yes, I want anyone to be able to issue transactions on my blockchain", nil).Once()

				// Contract deployment permissions
				contractOptions := []string{
					"Yes, I want anyone to be able to deploy smart contracts on my blockchain",
					"No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("Yes, I want anyone to be able to deploy smart contracts on my blockchain", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile)
				require.False(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "user explains contract deployment option then restricts",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				// Transaction permissions
				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("Yes, I want anyone to be able to issue transactions on my blockchain", nil)

				// Contract deployment permissions with explain
				contractOptions := []string{
					"Yes, I want anyone to be able to deploy smart contracts on my blockchain",
					"No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)",
					"Explain the difference",
				}
				// First call: user selects explain
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("Explain the difference", nil).Once()
				// Second call: user selects restrict
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)", nil).Once()

				// Mock GenerateAllowList for contract deployment
				contractAllowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to deploy smart contracts", contractAllowListOptions).Return("Confirm Allow List", nil)
				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile)
				require.True(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "main prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				options := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", options).Return("", errors.New("main prompt failed"))
			},
			expectedError: "main prompt failed",
			validateResult: func(_ *testing.T, _ SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "transaction prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("", errors.New("transaction prompt failed"))
			},
			expectedError: "transaction prompt failed",
			validateResult: func(_ *testing.T, _ SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "transaction GenerateAllowList fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)", nil)

				// Mock GenerateAllowList failure
				transactionAllowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to issue transactions", transactionAllowListOptions).Return("", errors.New("transaction allow list failed"))
			},
			expectedError: "transaction allow list failed",
			validateResult: func(_ *testing.T, _ SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "contract deployment prompt fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("Yes, I want anyone to be able to issue transactions on my blockchain", nil)

				contractOptions := []string{
					"Yes, I want anyone to be able to deploy smart contracts on my blockchain",
					"No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("", errors.New("contract deployment prompt failed"))
			},
			expectedError: "contract deployment prompt failed",
			validateResult: func(_ *testing.T, _ SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "contract deployment GenerateAllowList fails",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("Yes, I want anyone to be able to issue transactions on my blockchain", nil)

				contractOptions := []string{
					"Yes, I want anyone to be able to deploy smart contracts on my blockchain",
					"No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)", nil)

				// Mock GenerateAllowList failure
				contractAllowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to deploy smart contracts", contractAllowListOptions).Return("", errors.New("contract allow list failed"))
			},
			expectedError: "contract allow list failed",
			validateResult: func(_ *testing.T, _ SubnetEVMGenesisParams) {
				// Should not be called due to error
			},
		},
		{
			name:          "transaction GenerateAllowList cancelled and retried",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				// First time: user selects to restrict transactions
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)", nil).Once()

				// Mock GenerateAllowList - first call cancelled
				transactionAllowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to issue transactions", transactionAllowListOptions).Return("Cancel", nil).Once()

				// Second time: user allows anyone for transactions
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("Yes, I want anyone to be able to issue transactions on my blockchain", nil).Once()

				contractOptions := []string{
					"Yes, I want anyone to be able to deploy smart contracts on my blockchain",
					"No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("Yes, I want anyone to be able to deploy smart contracts on my blockchain", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile) // Should be false since user cancelled and then allowed anyone
				require.False(t, params.enableContractDeployerPrecompile)
			},
		},
		{
			name:          "contract deployment GenerateAllowList cancelled and retried",
			version:       "v0.6.8",
			defaultsKind:  NoDefaults,
			initialParams: SubnetEVMGenesisParams{},
			mockSetup: func(m *promptMocks.Prompter) {
				mainOptions := []string{"Yes", "No", "Explain the difference"}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions and deploy smart contracts to your blockchain?", mainOptions).Return("No", nil)

				transactionOptions := []string{
					"Yes, I want anyone to be able to issue transactions on my blockchain",
					"No, I want only approved addresses to issue transactions on my blockchain (Transaction Allow List ON)",
					"Explain the difference",
				}
				m.On("CaptureList", "Do you want to enable anyone to issue transactions to your blockchain?", transactionOptions).Return("Yes, I want anyone to be able to issue transactions on my blockchain", nil)

				contractOptions := []string{
					"Yes, I want anyone to be able to deploy smart contracts on my blockchain",
					"No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)",
					"Explain the difference",
				}
				// First time: user selects to restrict contract deployment
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)", nil).Once()

				// Mock GenerateAllowList - first call cancelled
				contractAllowListOptions := []string{
					"Add an address for a role to the allow list",
					"Preview Allow List",
					"Confirm Allow List",
					"Cancel",
				}
				m.On("CaptureList", "Configure the addresses that are allowed to deploy smart contracts", contractAllowListOptions).Return("Cancel", nil).Once()

				// Second time: user selects to restrict contract deployment again
				m.On("CaptureList", "Do you want to enable anyone to deploy smart contracts on your blockchain?", contractOptions).Return("No, I want only approved addresses to deploy smart contracts on my blockchain (Smart Contract Deployer Allow List ON)", nil).Once()

				// Mock GenerateAllowList - second call succeeds
				m.On("CaptureList", "Configure the addresses that are allowed to deploy smart contracts", contractAllowListOptions).Return("Confirm Allow List", nil).Once()
				confirmOptions := []string{"Yes", "No, keep editing"}
				m.On("CaptureList", "Confirm?", confirmOptions).Return("Yes", nil).Once()
			},
			expectedError: "",
			validateResult: func(t *testing.T, params SubnetEVMGenesisParams) {
				require.False(t, params.enableTransactionPrecompile)
				require.True(t, params.enableContractDeployerPrecompile) // Should be true since user succeeded on retry
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock prompter
			mockPrompter := promptMocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application with mock prompter
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Call the function under test
			resultParams, err := promptPermissioning(
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

func TestPromptUserForSubnetEVMVersion(t *testing.T) {
	tests := []struct {
		name           string
		mockSetup      func(*promptMocks.Prompter)
		mockDownloader func(*mocks.Downloader)
		expectedError  string
		validateResult func(*testing.T, string)
	}{
		{
			name: "user selects latest release version",
			mockSetup: func(m *promptMocks.Prompter) {
				// Debug: allow both 2 and 3 options to see which one actually gets called
				versionOptions2 := []string{"Use latest release version", "Specify custom version"}
				versionOptions3 := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions2).Return("Use latest release version", nil).Maybe()
				m.On("CaptureList", "Version", versionOptions3).Return("Use latest release version", nil).Maybe()
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock with identical release and pre-release versions - note the correct JSON field name
				depResponseJSON := testSubnetEVMVersion074
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.4", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.7.4", result, "Expected the mocked latest release version")
				t.Logf("Successfully retrieved latest release version: %s", result)
			},
		},
		{
			name: "user selects latest pre-release version (when different from release)",
			mockSetup: func(m *promptMocks.Prompter) {
				// Mock for when pre-release is different from release, so we get 3 options
				versionOptions := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions).Return("Use latest pre-release version", nil)
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock with different pre-release and release versions
				depResponseJSON := testSubnetEVMVersion074
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.5-rc1", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.7.5-rc1", result, "Expected the mocked pre-release version")
				t.Logf("Successfully retrieved version: %s", result)
			},
		},
		{
			name: "user selects custom version",
			mockSetup: func(m *promptMocks.Prompter) {
				// Debug: allow both 2 and 3 options to see which one actually gets called
				versionOptions2 := []string{"Use latest release version", "Specify custom version"}
				versionOptions3 := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions2).Return("Specify custom version", nil).Maybe()
				m.On("CaptureList", "Version", versionOptions3).Return("Specify custom version", nil).Maybe()

				// Mock the custom version list selection
				expectedVersions := []string{"v0.7.4", "v0.7.3", "v0.7.2", "v0.6.0", "v0.5.0"}
				m.On("CaptureList", "Pick the version for this VM", expectedVersions).Return("v0.6.0", nil)
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock with same release and pre-release versions to get 2 options - note the correct JSON field name
				depResponseJSON := testSubnetEVMVersion074
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.4", nil)

				// Mock the version list for custom selection
				versions := []string{"v0.7.4", "v0.7.3", "v0.7.2", "v0.6.0", "v0.5.0"}
				m.On("GetAllReleasesForRepo", "ava-labs", "subnet-evm", "", application.All).Return(versions, nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.6.0", result, "Expected the selected custom version")
				t.Logf("Successfully retrieved custom version: %s", result)
			},
		},
		{
			name: "user selects custom version from full list",
			mockSetup: func(m *promptMocks.Prompter) {
				// Debug: allow both 2 and 3 options to see which one actually gets called
				versionOptions2 := []string{"Use latest release version", "Specify custom version"}
				versionOptions3 := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions2).Return("Specify custom version", nil).Maybe()
				m.On("CaptureList", "Version", versionOptions3).Return("Specify custom version", nil).Maybe()

				// Mock the custom version selection from the full list
				expectedVersions := []string{"v0.7.4", "v0.7.3", "v0.7.2", "v0.7.1", "v0.7.0"}
				m.On("CaptureList", "Pick the version for this VM", expectedVersions).Return("v0.7.0", nil)
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock with same release and pre-release versions to get 2 options - note the correct JSON field name
				depResponseJSON := testSubnetEVMVersion074
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.4", nil)

				// Mock the version list for custom selection
				versions := []string{"v0.7.4", "v0.7.3", "v0.7.2", "v0.7.1", "v0.7.0"}
				m.On("GetAllReleasesForRepo", "ava-labs", "subnet-evm", "", application.All).Return(versions, nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.7.0", result, "Expected the selected custom version")
				t.Logf("Successfully retrieved custom version from full list: %s", result)
			},
		},
		{
			name: "version selection prompt fails",
			mockSetup: func(m *promptMocks.Prompter) {
				versionOptions := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions).Return("", errors.New("version selection failed"))
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock the dependency calls that happen before the prompt
				depResponseJSON := `{"subnet-evm": "v0.7.4"}`
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.5-rc1", nil) // Different from release to trigger 3 options
			},
			expectedError: "version selection failed",
			validateResult: func(_ *testing.T, _ string) {
				// Should not be called due to error
			},
		},
		{
			name: "custom version list prompt fails",
			mockSetup: func(m *promptMocks.Prompter) {
				// Debug: allow both 2 and 3 options to see which one actually gets called
				versionOptions2 := []string{"Use latest release version", "Specify custom version"}
				versionOptions3 := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions2).Return("Specify custom version", nil).Maybe()
				m.On("CaptureList", "Version", versionOptions3).Return("Specify custom version", nil).Maybe()

				// Mock the custom version selection to fail
				expectedVersions := []string{"v0.7.4", "v0.7.3", "v0.7.2", "v0.7.1", "v0.7.0"}
				m.On("CaptureList", "Pick the version for this VM", expectedVersions).Return("", errors.New("custom version selection failed"))
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock the network calls up to the point where the prompt fails - note the correct JSON field name
				depResponseJSON := testSubnetEVMVersion074
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.4", nil)

				// Mock the version list for custom selection
				versions := []string{"v0.7.4", "v0.7.3", "v0.7.2", "v0.7.1", "v0.7.0"}
				m.On("GetAllReleasesForRepo", "ava-labs", "subnet-evm", "", application.All).Return(versions, nil)
			},
			expectedError: "custom version selection failed",
			validateResult: func(_ *testing.T, _ string) {
				// Should not be called due to error
			},
		},
		{
			name: "user selects latest pre-release when different from release",
			mockSetup: func(m *promptMocks.Prompter) {
				// Mock for scenario where pre-release differs from release, resulting in 3 options
				versionOptions := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions).Return("Use latest pre-release version", nil)
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock with different pre-release and release versions
				depResponseJSON := testSubnetEVMVersion073
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.4-beta1", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.7.4-beta1", result, "Expected the mocked pre-release version")
				t.Logf("Successfully retrieved version (pre-release test): %s", result)
			},
		},
		{
			name: "user explicitly selects latest pre-release version",
			mockSetup: func(m *promptMocks.Prompter) {
				// Force 3 options by making pre-release different from release
				versionOptions := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions).Return("Use latest pre-release version", nil)
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock to ensure pre-release is different from release version
				depResponseJSON := testSubnetEVMVersion073
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.4-rc1", nil) // Different from release
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.7.4-rc1", result, "Expected the pre-release version to be returned")
				require.Contains(t, result, "rc", "Expected result to contain pre-release identifier")
				t.Logf("Successfully retrieved latest pre-release version: %s", result)
			},
		},
		{
			name: "offline mode uses hardcoded evm.Version",
			mockSetup: func(m *promptMocks.Prompter) {
				// In offline mode, both release and pre-release versions are the same (evm.Version)
				// so we expect only 2 options
				versionOptions := []string{"Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions).Return("Use latest release version", nil)
			},
			mockDownloader: func(_ *mocks.Downloader) {
				// No network calls should be made in offline mode
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.NotEmpty(t, result, "Expected non-empty version string")
				require.Regexp(t, `^v\d+\.\d+\.\d+`, result, "Expected version string to match semantic version pattern")
				t.Logf("Successfully retrieved offline version: %s", result)
			},
		},
		// New test cases with mocked downloader for network error scenarios
		{
			name: "GetLatestCLISupportedDependencyVersion fails",
			mockSetup: func(_ *promptMocks.Prompter) {
				// The prompt shouldn't be called since the error occurs before it
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock Download to fail for GetLatestCLISupportedDependencyVersion
				m.On("Download", mock.AnythingOfType("string")).Return([]byte{}, errors.New("network download failed"))
			},
			expectedError: "network download failed",
			validateResult: func(_ *testing.T, _ string) {
				// Should not be called due to error
			},
		},
		{
			name: "GetLatestPreReleaseVersion fails",
			mockSetup: func(_ *promptMocks.Prompter) {
				// The prompt shouldn't be called since the error occurs before it
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock successful Download for GetLatestCLISupportedDependencyVersion
				depResponseJSON := testSubnetEVMVersion074
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				// Mock GetLatestPreReleaseVersion to fail
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("", errors.New("pre-release fetch failed"))
			},
			expectedError: "pre-release fetch failed",
			validateResult: func(_ *testing.T, _ string) {
				// Should not be called due to error
			},
		},
		{
			name: "GetAllReleasesForRepo fails",
			mockSetup: func(m *promptMocks.Prompter) {
				versionOptions := []string{"Use latest pre-release version", "Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions).Return("Specify custom version", nil)
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock successful calls for the version options with different versions to force 3 options
				depResponseJSON := testSubnetEVMVersion073
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.4-rc1", nil) // Different from release to trigger 3 options
				// Mock GetAllReleasesForRepo to fail
				m.On("GetAllReleasesForRepo", "ava-labs", "subnet-evm", "", application.All).Return([]string{}, errors.New("failed to get releases"))
			},
			expectedError: "failed to get releases",
			validateResult: func(_ *testing.T, _ string) {
				// Should not be called due to error
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original environment variable value
			originalOfflineValue := os.Getenv(constants.OperateOfflineEnvVarName)
			defer func() {
				// Restore original environment variable value
				if originalOfflineValue == "" {
					os.Unsetenv(constants.OperateOfflineEnvVarName)
				} else {
					os.Setenv(constants.OperateOfflineEnvVarName, originalOfflineValue)
				}
			}()

			// Set offline mode for the specific test case
			if tt.name == "offline mode uses hardcoded evm.Version" {
				os.Setenv(constants.OperateOfflineEnvVarName, "true")
			}

			// Create a temporary directory for the app
			tempDir := t.TempDir()

			// Create mock prompter
			mockPrompter := promptMocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application instance with mock downloader
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			mockDownloader := mocks.NewDownloader(t)
			tt.mockDownloader(mockDownloader)
			app.Setup(
				tempDir,        // baseDir
				nil,            // logger not needed
				nil,            // config not needed
				"test",         // version
				mockPrompter,   // prompter
				mockDownloader, // mock downloader
				nil,            // cmd not needed
			)

			// Call the function under test
			result, err := promptUserForSubnetEVMVersion(app)

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

func TestPromptSubnetEVMVersion(t *testing.T) {
	tests := []struct {
		name             string
		subnetEVMVersion string
		mockSetup        func(*promptMocks.Prompter)
		mockDownloader   func(*mocks.Downloader)
		expectedError    string
		validateResult   func(*testing.T, string)
	}{
		{
			name:             "latest version - successful",
			subnetEVMVersion: "latest",
			mockSetup: func(_ *promptMocks.Prompter) {
				// No prompt expected for latest
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock GetLatestCLISupportedDependencyVersion
				depResponseJSON := `{"subnet-evm": "v0.7.5"}`
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.7.5", result, "Expected latest version from dependency")
				t.Logf("Successfully retrieved latest version: %s", result)
			},
		},
		{
			name:             "latest version - dependency error",
			subnetEVMVersion: "latest",
			mockSetup: func(_ *promptMocks.Prompter) {
				// No prompt expected for latest
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock GetLatestCLISupportedDependencyVersion to fail
				m.On("Download", mock.AnythingOfType("string")).Return([]byte{}, errors.New("dependency fetch failed"))
			},
			expectedError: "dependency fetch failed",
			validateResult: func(_ *testing.T, _ string) {
				// Should not be called due to error
			},
		},
		{
			name:             "pre-release version - successful",
			subnetEVMVersion: "pre-release",
			mockSetup: func(_ *promptMocks.Prompter) {
				// No prompt expected for pre-release
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock GetLatestPreReleaseVersion
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.6-rc1", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.7.6-rc1", result, "Expected pre-release version")
				t.Logf("Successfully retrieved pre-release version: %s", result)
			},
		},
		{
			name:             "pre-release version - downloader error",
			subnetEVMVersion: "pre-release",
			mockSetup: func(_ *promptMocks.Prompter) {
				// No prompt expected for pre-release
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock GetLatestPreReleaseVersion to fail
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("", errors.New("pre-release fetch failed"))
			},
			expectedError: "pre-release fetch failed",
			validateResult: func(_ *testing.T, _ string) {
				// Should not be called due to error
			},
		},
		{
			name:             "empty string - calls promptUserForSubnetEVMVersion",
			subnetEVMVersion: "",
			mockSetup: func(m *promptMocks.Prompter) {
				// Mock the version selection prompt (2 options since release == pre-release)
				versionOptions := []string{"Use latest release version", "Specify custom version"}
				m.On("CaptureList", "Version", versionOptions).Return("Use latest release version", nil)
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock the calls made by promptUserForSubnetEVMVersion
				depResponseJSON := testSubnetEVMVersion074
				m.On("Download", mock.AnythingOfType("string")).Return([]byte(depResponseJSON), nil)
				m.On("GetLatestPreReleaseVersion", "ava-labs", "subnet-evm", "").Return("v0.7.4", nil)
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.7.4", result, "Expected version from user prompt")
				t.Logf("Successfully retrieved version from user prompt: %s", result)
			},
		},
		{
			name:             "empty string - promptUserForSubnetEVMVersion error",
			subnetEVMVersion: "",
			mockSetup: func(_ *promptMocks.Prompter) {
				// No prompt setup - error will occur before prompting
			},
			mockDownloader: func(m *mocks.Downloader) {
				// Mock Download to fail for GetLatestCLISupportedDependencyVersion
				m.On("Download", mock.AnythingOfType("string")).Return([]byte{}, errors.New("network error"))
			},
			expectedError: "network error",
			validateResult: func(_ *testing.T, _ string) {
				// Should not be called due to error
			},
		},
		{
			name:             "custom version string - returned as-is",
			subnetEVMVersion: "v0.6.8",
			mockSetup: func(_ *promptMocks.Prompter) {
				// No prompt expected for custom version
			},
			mockDownloader: func(_ *mocks.Downloader) {
				// No downloader calls expected for custom version
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.6.8", result, "Expected custom version returned as-is")
				t.Logf("Successfully returned custom version: %s", result)
			},
		},
		{
			name:             "another custom version string - returned as-is",
			subnetEVMVersion: "v0.5.0",
			mockSetup: func(_ *promptMocks.Prompter) {
				// No prompt expected for custom version
			},
			mockDownloader: func(_ *mocks.Downloader) {
				// No downloader calls expected for custom version
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "v0.5.0", result, "Expected custom version returned as-is")
				t.Logf("Successfully returned custom version: %s", result)
			},
		},
		{
			name:             "random string - returned as-is",
			subnetEVMVersion: "custom-build-123",
			mockSetup: func(_ *promptMocks.Prompter) {
				// No prompt expected for custom version
			},
			mockDownloader: func(_ *mocks.Downloader) {
				// No downloader calls expected for custom version
			},
			expectedError: "",
			validateResult: func(t *testing.T, result string) {
				require.Equal(t, "custom-build-123", result, "Expected custom version returned as-is")
				t.Logf("Successfully returned custom version: %s", result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for the app
			tempDir := t.TempDir()

			// Create mock prompter
			mockPrompter := promptMocks.NewPrompter(t)

			// Set up mock expectations
			tt.mockSetup(mockPrompter)

			// Create application instance with mock downloader
			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			mockDownloader := mocks.NewDownloader(t)
			tt.mockDownloader(mockDownloader)
			app.Setup(
				tempDir,        // baseDir
				nil,            // logger not needed
				nil,            // config not needed
				"test",         // version
				mockPrompter,   // prompter
				mockDownloader, // mock downloader
				nil,            // cmd not needed
			)

			// Call the function under test
			result, err := PromptSubnetEVMVersion(app, tt.subnetEVMVersion)

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
