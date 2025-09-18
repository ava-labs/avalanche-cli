// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/testutils"
	"github.com/ava-labs/libevm/common"
	"github.com/ava-labs/subnet-evm/params/extras"
	"github.com/ava-labs/subnet-evm/precompile/allowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/deployerallowlist"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/stretchr/testify/require"
)

func TestAddAddressToAllowed(t *testing.T) {
	// Generate test addresses
	addrs, err := testutils.GenerateEthAddrs(6)
	require.NoError(t, err)

	type test struct {
		name                 string
		initialAllowList     allowlist.AllowListConfig
		addressToAdd         string
		expectedAdminCount   int
		expectedManagerCount int
		expectedEnabledCount int
		shouldAddToEnabled   bool
	}

	tests := []test{
		{
			name: "add address to empty allow list",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			addressToAdd:         "0x1234567890123456789012345678901234567890",
			expectedAdminCount:   0,
			expectedManagerCount: 0,
			expectedEnabledCount: 1,
			shouldAddToEnabled:   true,
		},
		{
			name: "add address not already in any list",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{addrs[0]},
				ManagerAddresses: []common.Address{addrs[1]},
				EnabledAddresses: []common.Address{addrs[2]},
			},
			addressToAdd:         addrs[3].Hex(),
			expectedAdminCount:   1,
			expectedManagerCount: 1,
			expectedEnabledCount: 2,
			shouldAddToEnabled:   true,
		},
		{
			name: "try to add address already in admin list",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{addrs[0], addrs[1]},
				ManagerAddresses: []common.Address{addrs[2]},
				EnabledAddresses: []common.Address{addrs[3]},
			},
			addressToAdd:         addrs[0].Hex(),
			expectedAdminCount:   2,
			expectedManagerCount: 1,
			expectedEnabledCount: 1,
			shouldAddToEnabled:   false,
		},
		{
			name: "try to add address already in manager list",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{addrs[0]},
				ManagerAddresses: []common.Address{addrs[1], addrs[2]},
				EnabledAddresses: []common.Address{addrs[3]},
			},
			addressToAdd:         addrs[1].Hex(),
			expectedAdminCount:   1,
			expectedManagerCount: 2,
			expectedEnabledCount: 1,
			shouldAddToEnabled:   false,
		},
		{
			name: "try to add address already in enabled list",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{addrs[0]},
				ManagerAddresses: []common.Address{addrs[1]},
				EnabledAddresses: []common.Address{addrs[2], addrs[3]},
			},
			addressToAdd:         addrs[2].Hex(),
			expectedAdminCount:   1,
			expectedManagerCount: 1,
			expectedEnabledCount: 2,
			shouldAddToEnabled:   false,
		},
		{
			name: "add address with lowercase hex format",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			addressToAdd:         "0xabcdef1234567890abcdef1234567890abcdef12",
			expectedAdminCount:   0,
			expectedManagerCount: 0,
			expectedEnabledCount: 1,
			shouldAddToEnabled:   true,
		},
		{
			name: "add address with uppercase hex format",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			addressToAdd:         "0xABCDEF1234567890ABCDEF1234567890ABCDEF12",
			expectedAdminCount:   0,
			expectedManagerCount: 0,
			expectedEnabledCount: 1,
			shouldAddToEnabled:   true,
		},
		{
			name: "add address without 0x prefix",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			addressToAdd:         "1234567890123456789012345678901234567890",
			expectedAdminCount:   0,
			expectedManagerCount: 0,
			expectedEnabledCount: 1,
			shouldAddToEnabled:   true,
		},
		{
			name: "case insensitive address comparison",
			initialAllowList: allowlist.AllowListConfig{
				AdminAddresses:   []common.Address{common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			addressToAdd:         "0xABCDEF1234567890ABCDEF1234567890ABCDEF12",
			expectedAdminCount:   1,
			expectedManagerCount: 0,
			expectedEnabledCount: 0,
			shouldAddToEnabled:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the initial allow list to avoid mutation
			initialConfig := allowlist.AllowListConfig{
				AdminAddresses:   make([]common.Address, len(tt.initialAllowList.AdminAddresses)),
				ManagerAddresses: make([]common.Address, len(tt.initialAllowList.ManagerAddresses)),
				EnabledAddresses: make([]common.Address, len(tt.initialAllowList.EnabledAddresses)),
			}
			copy(initialConfig.AdminAddresses, tt.initialAllowList.AdminAddresses)
			copy(initialConfig.ManagerAddresses, tt.initialAllowList.ManagerAddresses)
			copy(initialConfig.EnabledAddresses, tt.initialAllowList.EnabledAddresses)

			// Call the function
			result := addAddressToAllowed(initialConfig, tt.addressToAdd)

			// Verify the result
			require.Equal(t, tt.expectedAdminCount, len(result.AdminAddresses), "Admin addresses count mismatch")
			require.Equal(t, tt.expectedManagerCount, len(result.ManagerAddresses), "Manager addresses count mismatch")
			require.Equal(t, tt.expectedEnabledCount, len(result.EnabledAddresses), "Enabled addresses count mismatch")

			// Check if the address was added to enabled list when expected
			if tt.shouldAddToEnabled {
				expectedAddress := common.HexToAddress(tt.addressToAdd)
				found := false
				for _, addr := range result.EnabledAddresses {
					if addr == expectedAddress {
						found = true
						break
					}
				}
				require.True(t, found, "Address should have been added to enabled list")
			}

			// Verify original lists remain unchanged (admins and managers)
			require.Equal(t, len(tt.initialAllowList.AdminAddresses), len(result.AdminAddresses), "Admin list length should not change")
			require.Equal(t, len(tt.initialAllowList.ManagerAddresses), len(result.ManagerAddresses), "Manager list length should not change")
		})
	}
}

func TestAddICMAddressesToAllowLists(t *testing.T) {
	// Generate test addresses
	addrs, err := testutils.GenerateEthAddrs(5)
	require.NoError(t, err)

	type test struct {
		name                        string
		initialPrecompiles          extras.Precompiles
		icmAddress                  string
		icmMessengerDeployerAddress string
		relayerAddress              string
		expectedTxAllowListCount    int
		expectedDeployerListCount   int
		validateTxAllowList         func(t *testing.T, config *txallowlist.Config)
		validateDeployerAllowList   func(t *testing.T, config *deployerallowlist.Config)
	}

	tests := []test{
		{
			name:                        "add ICM addresses to empty precompiles",
			initialPrecompiles:          extras.Precompiles{},
			icmAddress:                  "0x1234567890123456789012345678901234567890",
			icmMessengerDeployerAddress: "0x2345678901234567890123456789012345678901",
			relayerAddress:              "0x3456789012345678901234567890123456789012",
			expectedTxAllowListCount:    0, // No tx allow list config
			expectedDeployerListCount:   0, // No deployer allow list config
		},
		{
			name: "add ICM addresses to precompiles with tx allow list only",
			initialPrecompiles: extras.Precompiles{
				txallowlist.ConfigKey: &txallowlist.Config{
					AllowListConfig: allowlist.AllowListConfig{
						AdminAddresses:   []common.Address{addrs[0]},
						ManagerAddresses: []common.Address{},
						EnabledAddresses: []common.Address{},
					},
				},
			},
			icmAddress:                  addrs[1].Hex(),
			icmMessengerDeployerAddress: addrs[2].Hex(),
			relayerAddress:              addrs[3].Hex(),
			expectedTxAllowListCount:    3, // 3 ICM addresses added to enabled
			expectedDeployerListCount:   0, // No deployer allow list config
			validateTxAllowList: func(t *testing.T, config *txallowlist.Config) {
				require.Equal(t, 1, len(config.AllowListConfig.AdminAddresses))
				require.Equal(t, 0, len(config.AllowListConfig.ManagerAddresses))
				require.Equal(t, 3, len(config.AllowListConfig.EnabledAddresses))

				// Check that all ICM addresses are in enabled list
				enabledMap := make(map[common.Address]bool)
				for _, addr := range config.AllowListConfig.EnabledAddresses {
					enabledMap[addr] = true
				}
				require.True(t, enabledMap[addrs[1]], "ICM address should be in enabled list")
				require.True(t, enabledMap[addrs[2]], "ICM messenger deployer address should be in enabled list")
				require.True(t, enabledMap[addrs[3]], "Relayer address should be in enabled list")
			},
		},
		{
			name: "add ICM addresses to precompiles with deployer allow list only",
			initialPrecompiles: extras.Precompiles{
				deployerallowlist.ConfigKey: &deployerallowlist.Config{
					AllowListConfig: allowlist.AllowListConfig{
						AdminAddresses:   []common.Address{addrs[0]},
						ManagerAddresses: []common.Address{},
						EnabledAddresses: []common.Address{},
					},
				},
			},
			icmAddress:                  addrs[1].Hex(),
			icmMessengerDeployerAddress: addrs[2].Hex(),
			relayerAddress:              addrs[3].Hex(),
			expectedTxAllowListCount:    0, // No tx allow list config
			expectedDeployerListCount:   2, // ICM and messenger deployer addresses added
			validateDeployerAllowList: func(t *testing.T, config *deployerallowlist.Config) {
				require.Equal(t, 1, len(config.AllowListConfig.AdminAddresses))
				require.Equal(t, 0, len(config.AllowListConfig.ManagerAddresses))
				require.Equal(t, 2, len(config.AllowListConfig.EnabledAddresses))

				// Check that ICM and messenger deployer addresses are in enabled list
				enabledMap := make(map[common.Address]bool)
				for _, addr := range config.AllowListConfig.EnabledAddresses {
					enabledMap[addr] = true
				}
				require.True(t, enabledMap[addrs[1]], "ICM address should be in enabled list")
				require.True(t, enabledMap[addrs[2]], "ICM messenger deployer address should be in enabled list")
				require.False(t, enabledMap[addrs[3]], "Relayer address should NOT be in deployer enabled list")
			},
		},
		{
			name: "add ICM addresses to precompiles with both allow lists",
			initialPrecompiles: extras.Precompiles{
				txallowlist.ConfigKey: &txallowlist.Config{
					AllowListConfig: allowlist.AllowListConfig{
						AdminAddresses:   []common.Address{addrs[0]},
						ManagerAddresses: []common.Address{},
						EnabledAddresses: []common.Address{addrs[4]},
					},
				},
				deployerallowlist.ConfigKey: &deployerallowlist.Config{
					AllowListConfig: allowlist.AllowListConfig{
						AdminAddresses:   []common.Address{addrs[0]},
						ManagerAddresses: []common.Address{},
						EnabledAddresses: []common.Address{},
					},
				},
			},
			icmAddress:                  addrs[1].Hex(),
			icmMessengerDeployerAddress: addrs[2].Hex(),
			relayerAddress:              addrs[3].Hex(),
			expectedTxAllowListCount:    4, // 1 existing + 3 ICM addresses
			expectedDeployerListCount:   2, // ICM and messenger deployer addresses added
			validateTxAllowList: func(t *testing.T, config *txallowlist.Config) {
				require.Equal(t, 1, len(config.AllowListConfig.AdminAddresses))
				require.Equal(t, 0, len(config.AllowListConfig.ManagerAddresses))
				require.Equal(t, 4, len(config.AllowListConfig.EnabledAddresses))

				// Check that all addresses are in enabled list
				enabledMap := make(map[common.Address]bool)
				for _, addr := range config.AllowListConfig.EnabledAddresses {
					enabledMap[addr] = true
				}
				require.True(t, enabledMap[addrs[4]], "Original enabled address should remain")
				require.True(t, enabledMap[addrs[1]], "ICM address should be in enabled list")
				require.True(t, enabledMap[addrs[2]], "ICM messenger deployer address should be in enabled list")
				require.True(t, enabledMap[addrs[3]], "Relayer address should be in enabled list")
			},
			validateDeployerAllowList: func(t *testing.T, config *deployerallowlist.Config) {
				require.Equal(t, 1, len(config.AllowListConfig.AdminAddresses))
				require.Equal(t, 0, len(config.AllowListConfig.ManagerAddresses))
				require.Equal(t, 2, len(config.AllowListConfig.EnabledAddresses))

				enabledMap := make(map[common.Address]bool)
				for _, addr := range config.AllowListConfig.EnabledAddresses {
					enabledMap[addr] = true
				}
				require.True(t, enabledMap[addrs[1]], "ICM address should be in enabled list")
				require.True(t, enabledMap[addrs[2]], "ICM messenger deployer address should be in enabled list")
				require.False(t, enabledMap[addrs[3]], "Relayer address should NOT be in deployer enabled list")
			},
		},
		{
			name: "add ICM addresses already in admin lists",
			initialPrecompiles: extras.Precompiles{
				txallowlist.ConfigKey: &txallowlist.Config{
					AllowListConfig: allowlist.AllowListConfig{
						AdminAddresses:   []common.Address{addrs[1], addrs[2]}, // ICM addresses already admin
						ManagerAddresses: []common.Address{},
						EnabledAddresses: []common.Address{},
					},
				},
				deployerallowlist.ConfigKey: &deployerallowlist.Config{
					AllowListConfig: allowlist.AllowListConfig{
						AdminAddresses:   []common.Address{addrs[1]}, // ICM address already admin
						ManagerAddresses: []common.Address{addrs[2]}, // Messenger deployer already manager
						EnabledAddresses: []common.Address{},
					},
				},
			},
			icmAddress:                  addrs[1].Hex(),
			icmMessengerDeployerAddress: addrs[2].Hex(),
			relayerAddress:              addrs[3].Hex(),
			expectedTxAllowListCount:    1, // Only relayer added to enabled
			expectedDeployerListCount:   0, // ICM and messenger deployer already have higher privileges
			validateTxAllowList: func(t *testing.T, config *txallowlist.Config) {
				require.Equal(t, 2, len(config.AllowListConfig.AdminAddresses))
				require.Equal(t, 0, len(config.AllowListConfig.ManagerAddresses))
				require.Equal(t, 1, len(config.AllowListConfig.EnabledAddresses))
				require.Equal(t, addrs[3], config.AllowListConfig.EnabledAddresses[0])
			},
			validateDeployerAllowList: func(t *testing.T, config *deployerallowlist.Config) {
				require.Equal(t, 1, len(config.AllowListConfig.AdminAddresses))
				require.Equal(t, 1, len(config.AllowListConfig.ManagerAddresses))
				require.Equal(t, 0, len(config.AllowListConfig.EnabledAddresses))
			},
		},
		{
			name: "add duplicate ICM addresses",
			initialPrecompiles: extras.Precompiles{
				txallowlist.ConfigKey: &txallowlist.Config{
					AllowListConfig: allowlist.AllowListConfig{
						AdminAddresses:   []common.Address{},
						ManagerAddresses: []common.Address{},
						EnabledAddresses: []common.Address{},
					},
				},
			},
			icmAddress:                  "0x1234567890123456789012345678901234567890",
			icmMessengerDeployerAddress: "0x1234567890123456789012345678901234567890", // Same as ICM address
			relayerAddress:              "0x1234567890123456789012345678901234567890", // Same as ICM address
			expectedTxAllowListCount:    1,                                            // Only one unique address should be added
			expectedDeployerListCount:   0,                                            // No deployer allow list
			validateTxAllowList: func(t *testing.T, config *txallowlist.Config) {
				require.Equal(t, 0, len(config.AllowListConfig.AdminAddresses))
				require.Equal(t, 0, len(config.AllowListConfig.ManagerAddresses))
				require.Equal(t, 1, len(config.AllowListConfig.EnabledAddresses))
				require.Equal(t, common.HexToAddress("0x1234567890123456789012345678901234567890"), config.AllowListConfig.EnabledAddresses[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the precompiles to avoid mutation
			precompilesCopy := make(extras.Precompiles)
			for key, value := range tt.initialPrecompiles {
				switch key {
				case txallowlist.ConfigKey:
					originalConfig := value.(*txallowlist.Config)
					copyConfig := &txallowlist.Config{
						AllowListConfig: allowlist.AllowListConfig{
							AdminAddresses:   make([]common.Address, len(originalConfig.AllowListConfig.AdminAddresses)),
							ManagerAddresses: make([]common.Address, len(originalConfig.AllowListConfig.ManagerAddresses)),
							EnabledAddresses: make([]common.Address, len(originalConfig.AllowListConfig.EnabledAddresses)),
						},
						Upgrade: originalConfig.Upgrade,
					}
					copy(copyConfig.AllowListConfig.AdminAddresses, originalConfig.AllowListConfig.AdminAddresses)
					copy(copyConfig.AllowListConfig.ManagerAddresses, originalConfig.AllowListConfig.ManagerAddresses)
					copy(copyConfig.AllowListConfig.EnabledAddresses, originalConfig.AllowListConfig.EnabledAddresses)
					precompilesCopy[key] = copyConfig
				case deployerallowlist.ConfigKey:
					originalConfig := value.(*deployerallowlist.Config)
					copyConfig := &deployerallowlist.Config{
						AllowListConfig: allowlist.AllowListConfig{
							AdminAddresses:   make([]common.Address, len(originalConfig.AllowListConfig.AdminAddresses)),
							ManagerAddresses: make([]common.Address, len(originalConfig.AllowListConfig.ManagerAddresses)),
							EnabledAddresses: make([]common.Address, len(originalConfig.AllowListConfig.EnabledAddresses)),
						},
						Upgrade: originalConfig.Upgrade,
					}
					copy(copyConfig.AllowListConfig.AdminAddresses, originalConfig.AllowListConfig.AdminAddresses)
					copy(copyConfig.AllowListConfig.ManagerAddresses, originalConfig.AllowListConfig.ManagerAddresses)
					copy(copyConfig.AllowListConfig.EnabledAddresses, originalConfig.AllowListConfig.EnabledAddresses)
					precompilesCopy[key] = copyConfig
				default:
					precompilesCopy[key] = value
				}
			}

			// Call the function
			addICMAddressesToAllowLists(
				&precompilesCopy,
				tt.icmAddress,
				tt.icmMessengerDeployerAddress,
				tt.relayerAddress,
			)

			// Validate tx allow list changes
			if txConfig, exists := precompilesCopy[txallowlist.ConfigKey]; exists {
				config := txConfig.(*txallowlist.Config)
				require.Equal(t, tt.expectedTxAllowListCount, len(config.AllowListConfig.EnabledAddresses), "Tx allow list enabled count mismatch")
				if tt.validateTxAllowList != nil {
					tt.validateTxAllowList(t, config)
				}
			} else {
				require.Equal(t, 0, tt.expectedTxAllowListCount, "Expected no tx allow list config")
			}

			// Validate deployer allow list changes
			if deployerConfig, exists := precompilesCopy[deployerallowlist.ConfigKey]; exists {
				config := deployerConfig.(*deployerallowlist.Config)
				require.Equal(t, tt.expectedDeployerListCount, len(config.AllowListConfig.EnabledAddresses), "Deployer allow list enabled count mismatch")
				if tt.validateDeployerAllowList != nil {
					tt.validateDeployerAllowList(t, config)
				}
			} else {
				require.Equal(t, 0, tt.expectedDeployerListCount, "Expected no deployer allow list config")
			}
		})
	}
}

func TestAddICMAddressesToAllowListsEdgeCases(t *testing.T) {
	addrs, err := testutils.GenerateEthAddrs(3)
	require.NoError(t, err)

	t.Run("panics with nil precompiles map", func(t *testing.T) {
		// The function actually panics when precompiles is nil - this is a limitation of the current implementation
		require.Panics(t, func() {
			var precompiles *extras.Precompiles
			addICMAddressesToAllowLists(
				precompiles,
				addrs[0].Hex(),
				addrs[1].Hex(),
				addrs[2].Hex(),
			)
		})
	})

	t.Run("handles empty address strings", func(t *testing.T) {
		precompiles := extras.Precompiles{
			txallowlist.ConfigKey: &txallowlist.Config{
				AllowListConfig: allowlist.AllowListConfig{
					AdminAddresses:   []common.Address{},
					ManagerAddresses: []common.Address{},
					EnabledAddresses: []common.Address{},
				},
			},
		}

		require.NotPanics(t, func() {
			addICMAddressesToAllowLists(
				&precompiles,
				"", // Empty ICM address
				"", // Empty messenger deployer address
				"", // Empty relayer address
			)
		})

		// Should have added 3 zero addresses
		txConfig := precompiles[txallowlist.ConfigKey].(*txallowlist.Config)
		require.Equal(t, 1, len(txConfig.AllowListConfig.EnabledAddresses)) // Only one zero address since they're all the same
		require.Equal(t, common.Address{}, txConfig.AllowListConfig.EnabledAddresses[0])
	})

	t.Run("handles invalid hex addresses", func(t *testing.T) {
		precompiles := extras.Precompiles{
			txallowlist.ConfigKey: &txallowlist.Config{
				AllowListConfig: allowlist.AllowListConfig{
					AdminAddresses:   []common.Address{},
					ManagerAddresses: []common.Address{},
					EnabledAddresses: []common.Address{},
				},
			},
		}

		require.NotPanics(t, func() {
			addICMAddressesToAllowLists(
				&precompiles,
				"invalid-hex-address",
				"also-invalid",
				"not-an-address",
			)
		})

		// Should have added zero addresses for invalid hex
		txConfig := precompiles[txallowlist.ConfigKey].(*txallowlist.Config)
		require.Equal(t, 1, len(txConfig.AllowListConfig.EnabledAddresses)) // Only one zero address since invalid addresses become zero
		require.Equal(t, common.Address{}, txConfig.AllowListConfig.EnabledAddresses[0])
	})
}

func TestAddAddressToAllowedEdgeCases(t *testing.T) {
	t.Run("handles empty address string", func(t *testing.T) {
		config := allowlist.AllowListConfig{
			AdminAddresses:   []common.Address{},
			ManagerAddresses: []common.Address{},
			EnabledAddresses: []common.Address{},
		}

		result := addAddressToAllowed(config, "")
		require.Equal(t, 1, len(result.EnabledAddresses))
		require.Equal(t, common.Address{}, result.EnabledAddresses[0])
	})

	t.Run("handles invalid hex address", func(t *testing.T) {
		config := allowlist.AllowListConfig{
			AdminAddresses:   []common.Address{},
			ManagerAddresses: []common.Address{},
			EnabledAddresses: []common.Address{},
		}

		result := addAddressToAllowed(config, "invalid-hex")
		require.Equal(t, 1, len(result.EnabledAddresses))
		require.Equal(t, common.Address{}, result.EnabledAddresses[0])
	})

	t.Run("preserves original slices content", func(t *testing.T) {
		addrs, err := testutils.GenerateEthAddrs(4)
		require.NoError(t, err)

		originalConfig := allowlist.AllowListConfig{
			AdminAddresses:   []common.Address{addrs[0]},
			ManagerAddresses: []common.Address{addrs[1]},
			EnabledAddresses: []common.Address{addrs[2]},
		}

		result := addAddressToAllowed(originalConfig, addrs[3].Hex())

		// Original lists should contain original addresses
		require.Contains(t, result.AdminAddresses, addrs[0])
		require.Contains(t, result.ManagerAddresses, addrs[1])
		require.Contains(t, result.EnabledAddresses, addrs[2])
		require.Contains(t, result.EnabledAddresses, addrs[3])
	})
}
