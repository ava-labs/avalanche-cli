// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/internal/testutils"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock variables for testing CreateEvmSidecar - these will override the variables in create_evm.go

func Test_ensureAdminsFunded(t *testing.T) {
	addrs, err := testutils.GenerateEthAddrs(5)
	require.NoError(t, err)

	type test struct {
		name       string
		alloc      core.GenesisAlloc
		allowList  AllowList
		shouldFail bool
	}
	tests := []test{
		{
			name: "One address funded",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[0]: {},
				addrs[1]: {
					Balance: big.NewInt(42),
				},
				addrs[2]: {},
			},
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[1]},
			},
			shouldFail: false,
		},
		{
			name: "Two addresses funded",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[2]: {},
				addrs[3]: {
					Balance: big.NewInt(42),
				},
				addrs[4]: {
					Balance: big.NewInt(42),
				},
			},
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[3], addrs[4]},
			},
			shouldFail: false,
		},
		{
			name: "Two addresses in Genesis but no funds",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[0]: {
					Balance: big.NewInt(0),
				},
				addrs[1]: {},
				addrs[2]: {},
			},
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[0], addrs[2]},
			},
			shouldFail: true,
		},
		{
			name: "No address funded",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[0]: {},
				addrs[1]: {},
				addrs[2]: {},
			},
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[3], addrs[4]},
			},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			b := someAllowedHasBalance(tt.allowList, tt.alloc)
			if tt.shouldFail {
				require.Equal(b, false)
			} else {
				require.Equal(b, true)
			}
		})
	}
}

func TestCreateEVMGenesis(t *testing.T) {
	// Helper to create test application with proper setup
	createTestApp := func(t *testing.T) *application.Avalanche {
		testDir := t.TempDir()
		app := &application.Avalanche{}

		// Create a mock prompter and downloader
		mockPrompter := prompts.NewPrompter()
		mockDownloader := mocks.NewDownloader(t)

		// Set up mock expectations - return default values for any call
		mockDownloader.On("Download", mock.Anything).Return([]byte("mock download"), nil).Maybe()
		mockDownloader.On("GetLatestReleaseVersion", mock.Anything, mock.Anything, mock.Anything).Return("v1.0.0", nil).Maybe()
		mockDownloader.On("GetLatestPreReleaseVersion", mock.Anything, mock.Anything, mock.Anything).Return("v1.0.0-rc1", nil).Maybe()
		mockDownloader.On("GetAllReleasesForRepo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{"v1.0.0", "v0.9.0"}, nil).Maybe()

		// Create a dummy command for app setup
		cmd := &cobra.Command{}

		// Setup the application with all required parameters
		app.Setup(testDir, logging.NoLog{}, &config.Config{}, "", mockPrompter, mockDownloader, cmd)

		// Create the necessary key directory and relayer key file for relayer operations
		keyDir := app.GetKeyDir()
		if err := os.MkdirAll(keyDir, 0o755); err != nil {
			t.Fatalf("Failed to create key directory: %v", err)
		}

		// Create a dummy relayer key file
		relayerKeyPath := filepath.Join(keyDir, "cli-awm-relayer.pk")
		dummyKeyContent := []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") // 64 hex chars = 32 bytes
		if err := os.WriteFile(relayerKeyPath, dummyKeyContent, 0o600); err != nil {
			t.Fatalf("Failed to create relayer key file: %v", err)
		}

		return app
	}

	// Helper to create basic genesis params
	createBasicGenesisParams := func() SubnetEVMGenesisParams {
		addrs, err := testutils.GenerateEthAddrs(3)
		require.NoError(t, err)

		return SubnetEVMGenesisParams{
			chainID: 12345,
			initialTokenAllocation: core.GenesisAlloc{
				addrs[0]: {Balance: big.NewInt(1000000000000000000)}, // 1 ETH
			},
			feeConfig: FeeConfig{
				lowThroughput: true,
			},
			enableWarpPrecompile: true,
		}
	}

	// Helper to create ICM info
	createICMInfo := func() *interchain.ICMInfo {
		return &interchain.ICMInfo{
			FundedAddress:            "0x1234567890123456789012345678901234567890",
			MessengerDeployerAddress: "0x1111111111111111111111111111111111111111",
			Version:                  "v1.0.0",
		}
	}

	type test struct {
		name                    string
		params                  SubnetEVMGenesisParams
		icmInfo                 *interchain.ICMInfo
		addICMRegistryToGenesis bool
		proxyOwner              string
		rewardBasisPoints       uint64
		useV2_0_0               bool
		expectedError           string
		validateGenesis         func(t *testing.T, genesisBytes []byte)
	}

	tests := []test{
		{
			name:                    "successful basic genesis creation",
			params:                  createBasicGenesisParams(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)

				// Check basic structure
				require.Contains(t, genesisMap, "config")
				require.Contains(t, genesisMap, "alloc")
				require.Contains(t, genesisMap, "timestamp")

				// Verify chain ID
				config := genesisMap["config"].(map[string]interface{})
				require.Equal(t, float64(12345), config["chainId"])
			},
		},
		{
			name: "nil initial token allocation",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.initialTokenAllocation = nil // Set to nil
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "genesis params allocation cannot be empty",
		},
		{
			name: "transaction precompile with allow list but no balance - should fail",
			params: func() SubnetEVMGenesisParams {
				addrs, err := testutils.GenerateEthAddrs(3)
				require.NoError(t, err)

				params := createBasicGenesisParams()
				params.enableTransactionPrecompile = true
				params.transactionPrecompileAllowList = AllowList{
					AdminAddresses: []common.Address{addrs[1]}, // Address not in allocation
				}
				// initialTokenAllocation only has addrs[0], not addrs[1]
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "none of the addresses in the transaction allow list precompile have any tokens allocated to them",
		},
		{
			name: "transaction precompile with allow list and balance - should succeed",
			params: func() SubnetEVMGenesisParams {
				addrs, err := testutils.GenerateEthAddrs(3)
				require.NoError(t, err)

				params := createBasicGenesisParams()
				// Override the initial allocation to use the same addresses
				params.initialTokenAllocation = core.GenesisAlloc{
					addrs[0]: {Balance: big.NewInt(1000000000000000000)}, // 1 ETH to addrs[0]
				}
				params.enableTransactionPrecompile = true
				params.transactionPrecompileAllowList = AllowList{
					AdminAddresses: []common.Address{addrs[0]}, // Same address as allocation
				}
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "",
		},
		{
			name: "ICM enabled but warp precompile disabled - should fail",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UseICM = true
				params.enableWarpPrecompile = false
				return params
			}(),
			icmInfo:                 createICMInfo(),
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "a ICM enabled blockchain was requested but warp precompile is disabled",
		},
		{
			name: "external gas token enabled but warp precompile disabled - should fail",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UseExternalGasToken = true
				params.enableWarpPrecompile = false
				return params
			}(),
			icmInfo:                 createICMInfo(),
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "a ICM enabled blockchain was requested but warp precompile is disabled",
		},
		{
			name: "ICM enabled but no ICM info provided - should fail",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UseICM = true
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "a ICM enabled blockchain was requested but no ICM info was provided",
		},
		{
			name: "external gas token enabled but no ICM info provided - should fail",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UseExternalGasToken = true
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "a ICM enabled blockchain was requested but no ICM info was provided",
		},
		{
			name: "both PoA and PoS enabled - should fail",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UsePoAValidatorManager = true
				params.UsePoSValidatorManager = true
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "blockchain can not be both PoA and PoS",
		},
		{
			name: "successful ICM enabled genesis",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UseICM = true
				return params
			}(),
			icmInfo:                 createICMInfo(),
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)

				// Verify ICM funded address is in allocation
				alloc := genesisMap["alloc"].(map[string]interface{})
				icmFundedAddr := "1234567890123456789012345678901234567890" // without 0x prefix
				require.Contains(t, alloc, icmFundedAddr)

				// Verify ICM balance is correct (600 AVAX)
				icmAccount := alloc[icmFundedAddr].(map[string]interface{})
				require.Contains(t, icmAccount, "balance")
				balance := icmAccount["balance"].(string)
				expectedBalance := "0x2086ac351052600000" // Updated to match actual value
				require.Equal(t, expectedBalance, balance)
			},
		},
		{
			name: "successful external gas token genesis",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UseExternalGasToken = true
				return params
			}(),
			icmInfo:                 createICMInfo(),
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)

				// Verify ICM funded address is in allocation with higher balance
				alloc := genesisMap["alloc"].(map[string]interface{})
				icmFundedAddr := "1234567890123456789012345678901234567890" // without 0x prefix
				require.Contains(t, alloc, icmFundedAddr)

				// Verify external gas token balance is correct (1000 AVAX)
				icmAccount := alloc[icmFundedAddr].(map[string]interface{})
				require.Contains(t, icmAccount, "balance")
				balance := icmAccount["balance"].(string)
				expectedBalance := "0x3635c9adc5dea00000" // Updated to match actual value
				require.Equal(t, expectedBalance, balance)
			},
		},
		{
			name: "successful PoA validator manager genesis",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UsePoAValidatorManager = true
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "0x2222222222222222222222222222222222222222",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)

				// Basic structure validation
				require.Contains(t, genesisMap, "alloc")
			},
		},
		{
			name: "successful PoS validator manager genesis",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UsePoSValidatorManager = true
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "0x3333333333333333333333333333333333333333",
			rewardBasisPoints:       100,
			useV2_0_0:               false,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)

				// Basic structure validation
				require.Contains(t, genesisMap, "alloc")
			},
		},
		{
			name: "v2.0.0 enabled for PoA",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UsePoAValidatorManager = true
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "0x4444444444444444444444444444444444444444",
			rewardBasisPoints:       0,
			useV2_0_0:               true,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)
			},
		},
		{
			name: "v2.0.0 enabled for PoS",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UsePoSValidatorManager = true
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "0x5555555555555555555555555555555555555555",
			rewardBasisPoints:       200,
			useV2_0_0:               true,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)

				// Basic structure validation
				require.Contains(t, genesisMap, "alloc")
				require.Contains(t, genesisMap, "config")
			},
		},
		{
			name: "ICM with registry at genesis",
			params: func() SubnetEVMGenesisParams {
				params := createBasicGenesisParams()
				params.UseICM = true
				return params
			}(),
			icmInfo:                 createICMInfo(),
			addICMRegistryToGenesis: true,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)
			},
		},
		{
			name: "complex genesis with multiple precompiles",
			params: func() SubnetEVMGenesisParams {
				addrs, err := testutils.GenerateEthAddrs(5)
				require.NoError(t, err)

				params := createBasicGenesisParams()
				// Ensure all addresses used in allow lists have proper funding
				params.initialTokenAllocation = core.GenesisAlloc{
					addrs[0]: {Balance: big.NewInt(1000000000000000000)}, // 1 ETH
					addrs[1]: {Balance: big.NewInt(1000000000000000000)}, // 1 ETH
					addrs[2]: {Balance: big.NewInt(1000000000000000000)}, // 1 ETH
					addrs[3]: {Balance: big.NewInt(1000000000000000000)}, // 1 ETH
				}
				params.enableNativeMinterPrecompile = true
				params.nativeMinterPrecompileAllowList = AllowList{
					AdminAddresses: []common.Address{addrs[0]},
				}
				params.enableFeeManagerPrecompile = true
				params.feeManagerPrecompileAllowList = AllowList{
					AdminAddresses: []common.Address{addrs[1]},
				}
				params.enableRewardManagerPrecompile = true
				params.rewardManagerPrecompileAllowList = AllowList{
					AdminAddresses: []common.Address{addrs[2]},
				}
				params.enableContractDeployerPrecompile = true
				params.contractDeployerPrecompileAllowList = AllowList{
					AdminAddresses: []common.Address{addrs[3]},
				}
				params.feeConfig = FeeConfig{
					mediumThroughput: true,
					useDynamicFees:   true,
				}
				return params
			}(),
			icmInfo:                 nil,
			addICMRegistryToGenesis: false,
			proxyOwner:              "",
			rewardBasisPoints:       0,
			useV2_0_0:               false,
			expectedError:           "",
			validateGenesis: func(t *testing.T, genesisBytes []byte) {
				require.NotEmpty(t, genesisBytes)

				// Verify it's valid JSON
				var genesisMap map[string]interface{}
				err := json.Unmarshal(genesisBytes, &genesisMap)
				require.NoError(t, err)

				// Verify config contains precompiles
				config := genesisMap["config"].(map[string]interface{})
				require.Contains(t, config, "contractNativeMinterConfig")      // Native minter
				require.Contains(t, config, "feeManagerConfig")                // Fee manager
				require.Contains(t, config, "rewardManagerConfig")             // Reward manager
				require.Contains(t, config, "contractDeployerAllowListConfig") // Contract deployer
				require.Contains(t, config, "warpConfig")                      // Warp
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestApp(t)

			genesisBytes, err := CreateEVMGenesis(
				app,
				tt.params,
				tt.icmInfo,
				tt.addICMRegistryToGenesis,
				tt.proxyOwner,
				tt.rewardBasisPoints,
				tt.useV2_0_0,
			)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, genesisBytes)

			if tt.validateGenesis != nil {
				tt.validateGenesis(t, genesisBytes)
			}
		})
	}
}

func TestSomeoneWasAllowed(t *testing.T) {
	addrs, err := testutils.GenerateEthAddrs(6)
	require.NoError(t, err)

	type test struct {
		name      string
		allowList AllowList
		expected  bool
	}
	tests := []test{
		{
			name:      "empty allow list",
			allowList: AllowList{},
			expected:  false,
		},
		{
			name: "only admin addresses",
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[0], addrs[1]},
			},
			expected: true,
		},
		{
			name: "only manager addresses",
			allowList: AllowList{
				ManagerAddresses: []common.Address{addrs[2]},
			},
			expected: true,
		},
		{
			name: "only enabled addresses",
			allowList: AllowList{
				EnabledAddresses: []common.Address{addrs[3], addrs[4]},
			},
			expected: true,
		},
		{
			name: "mixed addresses",
			allowList: AllowList{
				AdminAddresses:   []common.Address{addrs[0]},
				ManagerAddresses: []common.Address{addrs[1]},
				EnabledAddresses: []common.Address{addrs[2], addrs[3]},
			},
			expected: true,
		},
		{
			name: "single admin address",
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[5]},
			},
			expected: true,
		},
		{
			name: "single manager address",
			allowList: AllowList{
				ManagerAddresses: []common.Address{addrs[5]},
			},
			expected: true,
		},
		{
			name: "single enabled address",
			allowList: AllowList{
				EnabledAddresses: []common.Address{addrs[5]},
			},
			expected: true,
		},
		{
			name: "empty slices",
			allowList: AllowList{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := someoneWasAllowed(tt.allowList)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateEvmSidecar(t *testing.T) {
	// Store original functions to restore later
	originalSetupSubnetEVM := setupSubnetEVM
	originalGetVMBinaryProtocolVersion := getVMBinaryProtocolVersion
	originalGetRPCProtocolVersion := getRPCProtocolVersion
	defer func() {
		setupSubnetEVM = originalSetupSubnetEVM
		getVMBinaryProtocolVersion = originalGetVMBinaryProtocolVersion
		getRPCProtocolVersion = originalGetRPCProtocolVersion
	}()

	// Helper to create test application
	createTestApp := func(t *testing.T) *application.Avalanche {
		testDir := t.TempDir()
		app := &application.Avalanche{}
		mockPrompter := prompts.NewPrompter()
		mockDownloader := mocks.NewDownloader(t)

		// Set up mock expectations - return default values for any call
		mockDownloader.On("Download", mock.Anything).Return([]byte("mock download"), nil).Maybe()
		mockDownloader.On("GetLatestReleaseVersion", mock.Anything, mock.Anything, mock.Anything).Return("v1.0.0", nil).Maybe()
		mockDownloader.On("GetLatestPreReleaseVersion", mock.Anything, mock.Anything, mock.Anything).Return("v1.0.0-rc1", nil).Maybe()
		mockDownloader.On("GetAllReleasesForRepo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{"v1.0.0", "v0.9.0"}, nil).Maybe()

		cmd := &cobra.Command{}
		app.Setup(testDir, logging.NoLog{}, &config.Config{}, "", mockPrompter, mockDownloader, cmd)
		return app
	}

	type test struct {
		name                     string
		inputSidecar             *models.Sidecar
		subnetName               string
		subnetEVMVersion         string
		tokenSymbol              string
		getRPCVersionFromBinary  bool
		sovereign                bool
		useV2_0_0                bool
		setupSubnetEVMMock       func() func(*application.Avalanche, string) (string, string, error)
		setupVMBinaryVersionMock func() func(string) (int, error)
		setupRPCVersionMock      func() func(*application.Avalanche, models.VMType, string) (int, error)
		expectedError            string
		validateResult           func(t *testing.T, sidecar *models.Sidecar)
	}

	tests := []test{
		{
			name:                    "successful creation with nil input sidecar - from binary",
			inputSidecar:            nil,
			subnetName:              "test-subnet",
			subnetEVMVersion:        "v0.6.8",
			tokenSymbol:             "TEST",
			getRPCVersionFromBinary: true,
			sovereign:               true,
			useV2_0_0:               false,
			setupSubnetEVMMock: func() func(*application.Avalanche, string) (string, string, error) {
				return func(_ *application.Avalanche, _ string) (string, string, error) {
					return "subnet-evm-v0.6.8", "/path/to/subnet-evm", nil
				}
			},
			setupVMBinaryVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 28, nil
				}
			},
			setupRPCVersionMock: func() func(*application.Avalanche, models.VMType, string) (int, error) {
				return func(_ *application.Avalanche, _ models.VMType, _ string) (int, error) {
					return 0, nil // Not used in this test
				}
			},
			expectedError: "",
			validateResult: func(t *testing.T, sidecar *models.Sidecar) {
				require.NotNil(t, sidecar)
				require.Equal(t, "test-subnet", sidecar.Name)
				require.Equal(t, models.VMType(models.SubnetEvm), sidecar.VM)
				require.Equal(t, "v0.6.8", sidecar.VMVersion)
				require.Equal(t, 28, sidecar.RPCVersion)
				require.Equal(t, "test-subnet", sidecar.Subnet)
				require.Equal(t, "TEST", sidecar.TokenSymbol)
				require.Equal(t, "TEST Token", sidecar.TokenName)
				require.True(t, sidecar.Sovereign)
				require.False(t, sidecar.UseACP99)
			},
		},
		{
			name: "successful creation with existing sidecar - from RPC",
			inputSidecar: &models.Sidecar{
				Name:        "existing-subnet",
				TokenSymbol: "EXISTING",
			},
			subnetName:              "test-subnet",
			subnetEVMVersion:        "v0.6.9",
			tokenSymbol:             "NEW",
			getRPCVersionFromBinary: false,
			sovereign:               false,
			useV2_0_0:               true,
			setupSubnetEVMMock: func() func(*application.Avalanche, string) (string, string, error) {
				return func(_ *application.Avalanche, _ string) (string, string, error) {
					return "", "", nil // Not used in this test
				}
			},
			setupVMBinaryVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil // Not used in this test
				}
			},
			setupRPCVersionMock: func() func(*application.Avalanche, models.VMType, string) (int, error) {
				return func(_ *application.Avalanche, vmType models.VMType, version string) (int, error) {
					require.Equal(t, models.VMType(models.SubnetEvm), vmType)
					require.Equal(t, "v0.6.9", version)
					return 30, nil
				}
			},
			expectedError: "",
			validateResult: func(t *testing.T, sidecar *models.Sidecar) {
				require.NotNil(t, sidecar)
				require.Equal(t, "test-subnet", sidecar.Name) // Should be overwritten
				require.Equal(t, models.VMType(models.SubnetEvm), sidecar.VM)
				require.Equal(t, "v0.6.9", sidecar.VMVersion)
				require.Equal(t, 30, sidecar.RPCVersion)
				require.Equal(t, "test-subnet", sidecar.Subnet)
				require.Equal(t, "NEW", sidecar.TokenSymbol) // Should be overwritten
				require.Equal(t, "NEW Token", sidecar.TokenName)
				require.False(t, sidecar.Sovereign)
				require.True(t, sidecar.UseACP99)
			},
		},
		{
			name:                    "binutils.SetupSubnetEVM fails when getRPCVersionFromBinary is true",
			inputSidecar:            nil,
			subnetName:              "test-subnet",
			subnetEVMVersion:        "v0.6.8",
			tokenSymbol:             "TEST",
			getRPCVersionFromBinary: true,
			sovereign:               true,
			useV2_0_0:               false,
			setupSubnetEVMMock: func() func(*application.Avalanche, string) (string, string, error) {
				return func(_ *application.Avalanche, _ string) (string, string, error) {
					return "", "", fmt.Errorf("failed to download subnet-evm binary")
				}
			},
			setupVMBinaryVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil // Not called due to earlier failure
				}
			},
			setupRPCVersionMock: func() func(*application.Avalanche, models.VMType, string) (int, error) {
				return func(_ *application.Avalanche, _ models.VMType, _ string) (int, error) {
					return 0, nil // Not used in this test
				}
			},
			expectedError: "failed to install subnet-evm",
		},
		{
			name:                    "GetVMBinaryProtocolVersion fails when getRPCVersionFromBinary is true",
			inputSidecar:            nil,
			subnetName:              "test-subnet",
			subnetEVMVersion:        "v0.6.8",
			tokenSymbol:             "TEST",
			getRPCVersionFromBinary: true,
			sovereign:               false,
			useV2_0_0:               true,
			setupSubnetEVMMock: func() func(*application.Avalanche, string) (string, string, error) {
				return func(_ *application.Avalanche, _ string) (string, string, error) {
					return "subnet-evm-v0.6.8", "/path/to/subnet-evm", nil
				}
			},
			setupVMBinaryVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, fmt.Errorf("failed to get protocol version from binary")
				}
			},
			setupRPCVersionMock: func() func(*application.Avalanche, models.VMType, string) (int, error) {
				return func(_ *application.Avalanche, _ models.VMType, _ string) (int, error) {
					return 0, nil // Not used in this test
				}
			},
			expectedError: "unable to get RPC version",
		},
		{
			name:                    "GetRPCProtocolVersion fails when getRPCVersionFromBinary is false",
			inputSidecar:            nil,
			subnetName:              "test-subnet",
			subnetEVMVersion:        "v0.6.8",
			tokenSymbol:             "TEST",
			getRPCVersionFromBinary: false,
			sovereign:               true,
			useV2_0_0:               false,
			setupSubnetEVMMock: func() func(*application.Avalanche, string) (string, string, error) {
				return func(_ *application.Avalanche, _ string) (string, string, error) {
					return "", "", nil // Not used in this test
				}
			},
			setupVMBinaryVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil // Not used in this test
				}
			},
			setupRPCVersionMock: func() func(*application.Avalanche, models.VMType, string) (int, error) {
				return func(_ *application.Avalanche, _ models.VMType, _ string) (int, error) {
					return 0, fmt.Errorf("failed to get RPC protocol version")
				}
			},
			expectedError: "failed to get RPC protocol version",
		},
		{
			name:                    "successful creation with sovereign and v2.0.0 flags",
			inputSidecar:            nil,
			subnetName:              "sovereign-subnet",
			subnetEVMVersion:        "v0.7.0",
			tokenSymbol:             "SOV",
			getRPCVersionFromBinary: false,
			sovereign:               true,
			useV2_0_0:               true,
			setupSubnetEVMMock: func() func(*application.Avalanche, string) (string, string, error) {
				return func(_ *application.Avalanche, _ string) (string, string, error) {
					return "", "", nil // Not used in this test
				}
			},
			setupVMBinaryVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil // Not used in this test
				}
			},
			setupRPCVersionMock: func() func(*application.Avalanche, models.VMType, string) (int, error) {
				return func(_ *application.Avalanche, _ models.VMType, _ string) (int, error) {
					return 35, nil
				}
			},
			expectedError: "",
			validateResult: func(t *testing.T, sidecar *models.Sidecar) {
				require.NotNil(t, sidecar)
				require.Equal(t, "sovereign-subnet", sidecar.Name)
				require.Equal(t, models.VMType(models.SubnetEvm), sidecar.VM)
				require.Equal(t, "v0.7.0", sidecar.VMVersion)
				require.Equal(t, 35, sidecar.RPCVersion)
				require.Equal(t, "sovereign-subnet", sidecar.Subnet)
				require.Equal(t, "SOV", sidecar.TokenSymbol)
				require.Equal(t, "SOV Token", sidecar.TokenName)
				require.True(t, sidecar.Sovereign)
				require.True(t, sidecar.UseACP99)
			},
		},
		{
			name:                    "empty token symbol results in empty token name",
			inputSidecar:            nil,
			subnetName:              "empty-token-subnet",
			subnetEVMVersion:        "v0.6.8",
			tokenSymbol:             "",
			getRPCVersionFromBinary: false,
			sovereign:               false,
			useV2_0_0:               false,
			setupSubnetEVMMock: func() func(*application.Avalanche, string) (string, string, error) {
				return func(_ *application.Avalanche, _ string) (string, string, error) {
					return "", "", nil // Not used in this test
				}
			},
			setupVMBinaryVersionMock: func() func(string) (int, error) {
				return func(_ string) (int, error) {
					return 0, nil // Not used in this test
				}
			},
			setupRPCVersionMock: func() func(*application.Avalanche, models.VMType, string) (int, error) {
				return func(_ *application.Avalanche, _ models.VMType, _ string) (int, error) {
					return 25, nil
				}
			},
			expectedError: "",
			validateResult: func(t *testing.T, sidecar *models.Sidecar) {
				require.NotNil(t, sidecar)
				require.Equal(t, "", sidecar.TokenSymbol)
				require.Equal(t, " Token", sidecar.TokenName) // Empty symbol + " Token"
				require.False(t, sidecar.Sovereign)
				require.False(t, sidecar.UseACP99)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			setupSubnetEVM = tt.setupSubnetEVMMock()
			getVMBinaryProtocolVersion = tt.setupVMBinaryVersionMock()
			getRPCProtocolVersion = tt.setupRPCVersionMock()

			app := createTestApp(t)

			// Call the function
			result, err := CreateEvmSidecar(
				tt.inputSidecar,
				app,
				tt.subnetName,
				tt.subnetEVMVersion,
				tt.tokenSymbol,
				tt.getRPCVersionFromBinary,
				tt.sovereign,
				tt.useV2_0_0,
			)

			// Check error expectation
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}
