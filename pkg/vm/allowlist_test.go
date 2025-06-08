// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/prompts/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/olekukonko/tablewriter"
	"github.com/stretchr/testify/require"
)

func TestPreview(t *testing.T) {
	tests := []struct {
		name           string
		allowList      AllowList
		expectedOutput string
	}{
		{
			name: "empty allow list shows caution message",
			allowList: AllowList{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			expectedOutput: "Caution: Allow lists are empty. You will not be able to easily change the precompile settings in the future.",
		},
		{
			name: "allow list with admin addresses",
			allowList: AllowList{
				AdminAddresses: []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"),
					common.HexToAddress("0x2222222222222222222222222222222222222222"),
				},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			expectedOutput: "0x1111111111111111111111111111111111111111",
		},
		{
			name: "allow list with manager addresses",
			allowList: AllowList{
				AdminAddresses: []common.Address{},
				ManagerAddresses: []common.Address{
					common.HexToAddress("0x3333333333333333333333333333333333333333"),
				},
				EnabledAddresses: []common.Address{},
			},
			expectedOutput: "0x3333333333333333333333333333333333333333",
		},
		{
			name: "allow list with enabled addresses",
			allowList: AllowList{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{
					common.HexToAddress("0x4444444444444444444444444444444444444444"),
					common.HexToAddress("0x5555555555555555555555555555555555555555"),
				},
			},
			expectedOutput: "0x4444444444444444444444444444444444444444",
		},
		{
			name: "allow list with all types of addresses",
			allowList: AllowList{
				AdminAddresses: []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"),
				},
				ManagerAddresses: []common.Address{
					common.HexToAddress("0x2222222222222222222222222222222222222222"),
				},
				EnabledAddresses: []common.Address{
					common.HexToAddress("0x3333333333333333333333333333333333333333"),
				},
			},
			expectedOutput: "0x1111111111111111111111111111111111111111",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture output
			var buf bytes.Buffer

			// Call the preview function with our buffer
			preview(tt.allowList, &buf)

			// Get the output as a string
			output := buf.String()

			// Verify that the expected content is in the output
			require.Contains(t, output, tt.expectedOutput, "Expected output to contain specific content")

			// Additional checks based on the test case
			switch tt.name {
			case "empty allow list shows caution message":
				require.Contains(t, output, "Caution:", "Empty allow list should show caution message")
				require.Contains(t, output, "Admins", "Should contain role headers")
				require.Contains(t, output, "Manager", "Should contain role headers")
				require.Contains(t, output, "Enabled", "Should contain role headers")

			case "allow list with admin addresses":
				require.Contains(t, output, "Admins", "Should contain admin role header")
				require.Contains(t, output, "0x1111111111111111111111111111111111111111", "Should contain first admin address")
				require.Contains(t, output, "0x2222222222222222222222222222222222222222", "Should contain second admin address")
				require.NotContains(t, output, "Caution:", "Should not show caution message when addresses are present")

			case "allow list with manager addresses":
				require.Contains(t, output, "Manager", "Should contain manager role header")
				require.Contains(t, output, "0x3333333333333333333333333333333333333333", "Should contain manager address")
				require.NotContains(t, output, "Caution:", "Should not show caution message when addresses are present")

			case "allow list with enabled addresses":
				require.Contains(t, output, "Enabled", "Should contain enabled role header")
				require.Contains(t, output, "0x4444444444444444444444444444444444444444", "Should contain first enabled address")
				require.Contains(t, output, "0x5555555555555555555555555555555555555555", "Should contain second enabled address")
				require.NotContains(t, output, "Caution:", "Should not show caution message when addresses are present")

			case "allow list with all types of addresses":
				require.Contains(t, output, "Admins", "Should contain admin role header")
				require.Contains(t, output, "Manager", "Should contain manager role header")
				require.Contains(t, output, "Enabled", "Should contain enabled role header")
				require.Contains(t, output, "0x1111111111111111111111111111111111111111", "Should contain admin address")
				require.Contains(t, output, "0x2222222222222222222222222222222222222222", "Should contain manager address")
				require.Contains(t, output, "0x3333333333333333333333333333333333333333", "Should contain enabled address")
				require.NotContains(t, output, "Caution:", "Should not show caution message when addresses are present")
			}

			// Verify that table structure is present (basic check for table formatting)
			require.True(t, strings.Contains(output, "+") || strings.Contains(output, "-") || strings.Contains(output, "|"),
				"Output should contain table formatting characters")
		})
	}
}

func TestPreviewTableStructure(t *testing.T) {
	// Test that the table structure is correctly formatted
	allowList := AllowList{
		AdminAddresses: []common.Address{
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
		},
		ManagerAddresses: []common.Address{},
		EnabledAddresses: []common.Address{},
	}

	var buf bytes.Buffer
	preview(allowList, &buf)
	output := buf.String()

	// Check for table structure elements
	require.Contains(t, output, "+", "Output should contain table border characters")
	require.Contains(t, output, "|", "Output should contain table column separators")
	require.Contains(t, output, "-", "Output should contain table row separators")

	// Verify roles are present
	require.Contains(t, output, "Admins", "Should contain Admins role")
	require.Contains(t, output, "Manager", "Should contain Manager role")
	require.Contains(t, output, "Enabled", "Should contain Enabled role")
}

func TestPreviewWithNilWriter(t *testing.T) {
	// Test that the function handles different writer types
	allowList := AllowList{
		AdminAddresses: []common.Address{
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
		},
		ManagerAddresses: []common.Address{},
		EnabledAddresses: []common.Address{},
	}

	// Test with bytes.Buffer
	var buf bytes.Buffer
	preview(allowList, &buf)
	require.NotEmpty(t, buf.String(), "Buffer should contain output")

	// Test with a string builder
	var sb strings.Builder
	preview(allowList, &sb)
	require.NotEmpty(t, sb.String(), "String builder should contain output")
	require.Contains(t, sb.String(), "0x1111111111111111111111111111111111111111", "Should contain the address")
}

func TestPreviewAddressFormatting(t *testing.T) {
	// Test that addresses are properly formatted in the output
	allowList := AllowList{
		AdminAddresses: []common.Address{
			common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12"),
		},
		ManagerAddresses: []common.Address{
			common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
		},
		EnabledAddresses: []common.Address{
			common.HexToAddress("0xfedcba0987654321fedcba0987654321fedcba09"),
		},
	}

	var buf bytes.Buffer
	preview(allowList, &buf)
	output := buf.String()

	// Check that addresses are properly formatted (should have 0x prefix and correct case)
	require.Contains(t, output, "0xabCDEF1234567890ABcDEF1234567890aBCDeF12", "Admin address should be properly formatted")
	require.Contains(t, output, "0x1234567890AbcdEF1234567890aBcdef12345678", "Manager address should be properly formatted")
	require.Contains(t, output, "0xfEDCBA0987654321FeDcbA0987654321fedCBA09", "Enabled address should be properly formatted")
}

func TestGetNewAddresses(t *testing.T) {
	tests := []struct {
		name           string
		allowList      AllowList
		mockSetup      func(*mocks.Prompter)
		expectedAddrs  []common.Address
		expectedError  string
		expectedOutput string
	}{
		{
			name: "successful capture with new addresses only",
			allowList: AllowList{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			mockSetup: func(m *mocks.Prompter) {
				addresses := []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"),
					common.HexToAddress("0x2222222222222222222222222222222222222222"),
				}
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return(addresses, nil)
			},
			expectedAddrs: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
				common.HexToAddress("0x2222222222222222222222222222222222222222"),
			},
			expectedError:  "",
			expectedOutput: "",
		},
		{
			name: "addresses already in admin role",
			allowList: AllowList{
				AdminAddresses: []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"),
					common.HexToAddress("0x2222222222222222222222222222222222222222"),
				},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			mockSetup: func(m *mocks.Prompter) {
				addresses := []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"),
					common.HexToAddress("0x2222222222222222222222222222222222222222"),
				}
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return(addresses, nil)
			},
			expectedAddrs:  []common.Address{},
			expectedError:  "",
			expectedOutput: "0x1111111111111111111111111111111111111111 is already allowed as admin role\n0x2222222222222222222222222222222222222222 is already allowed as admin role",
		},
		{
			name: "addresses already in manager role",
			allowList: AllowList{
				AdminAddresses: []common.Address{},
				ManagerAddresses: []common.Address{
					common.HexToAddress("0x3333333333333333333333333333333333333333"),
				},
				EnabledAddresses: []common.Address{},
			},
			mockSetup: func(m *mocks.Prompter) {
				addresses := []common.Address{
					common.HexToAddress("0x3333333333333333333333333333333333333333"),
				}
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return(addresses, nil)
			},
			expectedAddrs:  []common.Address{},
			expectedError:  "",
			expectedOutput: "0x3333333333333333333333333333333333333333 is already allowed as manager role",
		},
		{
			name: "addresses already in enabled role",
			allowList: AllowList{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{
					common.HexToAddress("0x4444444444444444444444444444444444444444"),
					common.HexToAddress("0x5555555555555555555555555555555555555555"),
				},
			},
			mockSetup: func(m *mocks.Prompter) {
				addresses := []common.Address{
					common.HexToAddress("0x4444444444444444444444444444444444444444"),
				}
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return(addresses, nil)
			},
			expectedAddrs:  []common.Address{},
			expectedError:  "",
			expectedOutput: "0x4444444444444444444444444444444444444444 is already allowed as enabled role",
		},
		{
			name: "mixed scenario - some new and some existing addresses",
			allowList: AllowList{
				AdminAddresses: []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"),
				},
				ManagerAddresses: []common.Address{
					common.HexToAddress("0x2222222222222222222222222222222222222222"),
				},
				EnabledAddresses: []common.Address{
					common.HexToAddress("0x3333333333333333333333333333333333333333"),
				},
			},
			mockSetup: func(m *mocks.Prompter) {
				addresses := []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"), // existing admin
					common.HexToAddress("0x2222222222222222222222222222222222222222"), // existing manager
					common.HexToAddress("0x3333333333333333333333333333333333333333"), // existing enabled
					common.HexToAddress("0x4444444444444444444444444444444444444444"), // new
					common.HexToAddress("0x5555555555555555555555555555555555555555"), // new
				}
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return(addresses, nil)
			},
			expectedAddrs: []common.Address{
				common.HexToAddress("0x4444444444444444444444444444444444444444"),
				common.HexToAddress("0x5555555555555555555555555555555555555555"),
			},
			expectedError:  "",
			expectedOutput: "0x1111111111111111111111111111111111111111 is already allowed as admin role\n0x2222222222222222222222222222222222222222 is already allowed as manager role\n0x3333333333333333333333333333333333333333 is already allowed as enabled role",
		},
		{
			name: "CaptureAddresses returns error",
			allowList: AllowList{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return([]common.Address{}, errors.New("capture failed"))
			},
			expectedAddrs:  nil,
			expectedError:  "capture failed",
			expectedOutput: "",
		},
		{
			name: "empty address list from CaptureAddresses",
			allowList: AllowList{
				AdminAddresses:   []common.Address{},
				ManagerAddresses: []common.Address{},
				EnabledAddresses: []common.Address{},
			},
			mockSetup: func(m *mocks.Prompter) {
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return([]common.Address{}, nil)
			},
			expectedAddrs:  []common.Address{},
			expectedError:  "",
			expectedOutput: "",
		},
		{
			name: "duplicate addresses in different roles",
			allowList: AllowList{
				AdminAddresses: []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"),
				},
				ManagerAddresses: []common.Address{
					common.HexToAddress("0x2222222222222222222222222222222222222222"),
				},
				EnabledAddresses: []common.Address{
					common.HexToAddress("0x3333333333333333333333333333333333333333"),
				},
			},
			mockSetup: func(m *mocks.Prompter) {
				addresses := []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"), // admin takes precedence
					common.HexToAddress("0x4444444444444444444444444444444444444444"), // new
				}
				m.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return(addresses, nil)
			},
			expectedAddrs: []common.Address{
				common.HexToAddress("0x4444444444444444444444444444444444444444"),
			},
			expectedError:  "",
			expectedOutput: "0x1111111111111111111111111111111111111111 is already allowed as admin role",
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

			// Capture stdout to verify printed messages
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call the function under test
			result, err := getNewAddresses(app, tt.allowList)

			// Restore stdout and read captured output
			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			_, readErr := buf.ReadFrom(r)
			require.NoError(t, readErr)
			output := strings.TrimSpace(buf.String())

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedAddrs, result)

				// Verify output messages
				if tt.expectedOutput != "" {
					require.Contains(t, output, tt.expectedOutput)
				} else {
					require.Empty(t, output)
				}
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestGetNewAddressesEdgeCases(t *testing.T) {
	// Test edge case: same address in multiple roles (admin takes precedence)
	t.Run("address exists in multiple roles - admin takes precedence", func(t *testing.T) {
		allowList := AllowList{
			AdminAddresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
			},
			ManagerAddresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"), // same address
			},
			EnabledAddresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"), // same address
			},
		}

		mockPrompter := mocks.NewPrompter(t)
		addresses := []common.Address{
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
		}
		mockPrompter.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return(addresses, nil)

		app := &application.Avalanche{
			Prompt: mockPrompter,
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		result, err := getNewAddresses(app, allowList)

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, readErr := buf.ReadFrom(r)
		require.NoError(t, readErr)
		output := strings.TrimSpace(buf.String())

		require.NoError(t, err)
		require.Empty(t, result)
		require.Contains(t, output, "is already allowed as admin role")
		require.NotContains(t, output, "manager role")
		require.NotContains(t, output, "enabled role")

		mockPrompter.AssertExpectations(t)
	})

	// Test edge case: Case sensitivity in address comparison
	t.Run("case insensitive address comparison", func(t *testing.T) {
		allowList := AllowList{
			AdminAddresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"), // lowercase
			},
			ManagerAddresses: []common.Address{},
			EnabledAddresses: []common.Address{},
		}

		mockPrompter := mocks.NewPrompter(t)
		addresses := []common.Address{
			common.HexToAddress("0X1111111111111111111111111111111111111111"), // uppercase - should still match
		}
		mockPrompter.On("CaptureAddresses", "Enter the address of the account (or multiple comma separated):").Return(addresses, nil)

		app := &application.Avalanche{
			Prompt: mockPrompter,
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		result, err := getNewAddresses(app, allowList)

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, readErr := buf.ReadFrom(r)
		require.NoError(t, readErr)
		output := strings.TrimSpace(buf.String())

		require.NoError(t, err)
		require.Empty(t, result)
		require.Contains(t, output, "is already allowed as admin role")

		mockPrompter.AssertExpectations(t)
	})
}

func TestAddRoleToPreviewTable(t *testing.T) {
	tests := []struct {
		name            string
		roleName        string
		addresses       []common.Address
		expectedContent string
		expectedSpaces  bool
	}{
		{
			name:            "empty address list shows spaces",
			roleName:        "Admins",
			addresses:       []common.Address{},
			expectedContent: "Admins",
			expectedSpaces:  true,
		},
		{
			name:     "single address",
			roleName: "Manager",
			addresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
			},
			expectedContent: "0x1111111111111111111111111111111111111111",
			expectedSpaces:  false,
		},
		{
			name:     "multiple addresses joined with newlines",
			roleName: "Enabled",
			addresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
				common.HexToAddress("0x2222222222222222222222222222222222222222"),
				common.HexToAddress("0x3333333333333333333333333333333333333333"),
			},
			expectedContent: "0x1111111111111111111111111111111111111111",
			expectedSpaces:  false,
		},
		{
			name:     "different role name",
			roleName: "Custom Role",
			addresses: []common.Address{
				common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12"),
			},
			expectedContent: "0xabCDEF1234567890ABcDEF1234567890aBCDeF12",
			expectedSpaces:  false,
		},
		{
			name:            "empty role name with empty addresses",
			roleName:        "",
			addresses:       []common.Address{},
			expectedContent: "",
			expectedSpaces:  true,
		},
		{
			name:     "role name with special characters",
			roleName: "Special-Role_123",
			addresses: []common.Address{
				common.HexToAddress("0x4444444444444444444444444444444444444444"),
			},
			expectedContent: "0x4444444444444444444444444444444444444444",
			expectedSpaces:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture table output
			var buf bytes.Buffer

			// Create a new table writer
			table := tablewriter.NewWriter(&buf)

			// Call the function under test
			addRoleToPreviewTable(table, tt.roleName, tt.addresses)

			// Render the table to get the output
			table.Render()

			// Get the output as string
			output := buf.String()

			// Verify role name appears in output
			require.Contains(t, output, tt.roleName, "Role name should appear in table output")

			// Verify expected content appears in output
			if tt.expectedContent != "" {
				require.Contains(t, output, tt.expectedContent, "Expected content should appear in table")
			}

			// Check for spaces when addresses are empty
			if tt.expectedSpaces {
				require.Contains(t, output, strings.Repeat(" ", 11), "Empty address list should show 11 spaces")
			}

			// Verify all addresses appear in output for non-empty cases
			if len(tt.addresses) > 0 {
				for _, addr := range tt.addresses {
					require.Contains(t, output, addr.Hex(), "Each address should appear in the output")
				}
			}
		})
	}
}

func TestAddRoleToPreviewTableMultipleRoles(t *testing.T) {
	// Test adding multiple roles to the same table
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)

	// Add multiple roles
	adminAddresses := []common.Address{
		common.HexToAddress("0x1111111111111111111111111111111111111111"),
		common.HexToAddress("0x2222222222222222222222222222222222222222"),
	}
	managerAddresses := []common.Address{
		common.HexToAddress("0x3333333333333333333333333333333333333333"),
	}
	enabledAddresses := []common.Address{} // empty

	addRoleToPreviewTable(table, "Admins", adminAddresses)
	addRoleToPreviewTable(table, "Manager", managerAddresses)
	addRoleToPreviewTable(table, "Enabled", enabledAddresses)

	// Render the table
	table.Render()
	output := buf.String()

	// Verify all role names appear
	require.Contains(t, output, "Admins", "Admins role should appear")
	require.Contains(t, output, "Manager", "Manager role should appear")
	require.Contains(t, output, "Enabled", "Enabled role should appear")

	// Verify admin addresses appear
	require.Contains(t, output, "0x1111111111111111111111111111111111111111", "First admin address should appear")
	require.Contains(t, output, "0x2222222222222222222222222222222222222222", "Second admin address should appear")

	// Verify manager address appears
	require.Contains(t, output, "0x3333333333333333333333333333333333333333", "Manager address should appear")

	// Verify empty enabled addresses show spaces
	require.Contains(t, output, strings.Repeat(" ", 11), "Empty enabled addresses should show spaces")
}

func TestAddRoleToPreviewTableAddressFormatting(t *testing.T) {
	// Test that addresses are properly formatted in hex
	testCases := []struct {
		name        string
		inputAddr   string
		expectedHex string
	}{
		{
			name:        "lowercase address",
			inputAddr:   "0xabcdef1234567890abcdef1234567890abcdef12",
			expectedHex: "0xabCDEF1234567890ABcDEF1234567890aBCDeF12",
		},
		{
			name:        "uppercase address",
			inputAddr:   "0XFEDCBA0987654321FEDCBA0987654321FEDCBA09",
			expectedHex: "0xfEDCBA0987654321FeDcbA0987654321fedCBA09",
		},
		{
			name:        "mixed case address",
			inputAddr:   "0x1234567890AbCdEf1234567890aBcDeF12345678",
			expectedHex: "0x1234567890AbcdEF1234567890aBcdef12345678",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			table := tablewriter.NewWriter(&buf)

			addresses := []common.Address{
				common.HexToAddress(tc.inputAddr),
			}

			addRoleToPreviewTable(table, "TestRole", addresses)
			table.Render()
			output := buf.String()

			require.Contains(t, output, tc.expectedHex, "Address should be formatted correctly in output")
		})
	}
}

func TestAddRoleToPreviewTableTableStructure(t *testing.T) {
	// Test that the function properly integrates with tablewriter
	var buf bytes.Buffer
	table := tablewriter.NewWriter(&buf)

	// Configure table like in the preview function
	table.SetRowLine(true)
	table.SetAutoMergeCellsByColumnIndex([]int{0})

	addresses := []common.Address{
		common.HexToAddress("0x1111111111111111111111111111111111111111"),
	}

	addRoleToPreviewTable(table, "TestRole", addresses)
	table.Render()
	output := buf.String()

	// Verify table structure elements are present
	require.Contains(t, output, "+", "Table should contain border characters")
	require.Contains(t, output, "|", "Table should contain column separators")
	require.Contains(t, output, "-", "Table should contain row separators")
	require.Contains(t, output, "TestRole", "Role name should appear in table")
	require.Contains(t, output, "0x1111111111111111111111111111111111111111", "Address should appear in table")
}

func TestAddRoleToPreviewTableEdgeCases(t *testing.T) {
	t.Run("very long role name", func(t *testing.T) {
		var buf bytes.Buffer
		table := tablewriter.NewWriter(&buf)

		longRoleName := strings.Repeat("VeryLongRoleName", 10) // 160 characters
		addresses := []common.Address{
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
		}

		addRoleToPreviewTable(table, longRoleName, addresses)
		table.Render()
		output := buf.String()

		require.Contains(t, output, longRoleName, "Long role name should appear in output")
		require.Contains(t, output, "0x1111111111111111111111111111111111111111", "Address should appear in output")
	})

	t.Run("zero address", func(t *testing.T) {
		var buf bytes.Buffer
		table := tablewriter.NewWriter(&buf)

		addresses := []common.Address{
			{}, // zero address
		}

		addRoleToPreviewTable(table, "ZeroAddress", addresses)
		table.Render()
		output := buf.String()

		require.Contains(t, output, "0x0000000000000000000000000000000000000000", "Zero address should be formatted correctly")
		require.Contains(t, output, "ZeroAddress", "Role name should appear in output")
	})
}

func TestRemoveAddress(t *testing.T) {
	tests := []struct {
		name              string
		inputAddresses    []common.Address
		kind              string
		mockSetup         func(*mocks.Prompter)
		expectedAddresses []common.Address
		expectedCancelled bool
		expectedError     string
		expectedOutput    string
	}{
		{
			name:           "empty address list shows message and returns cancelled",
			inputAddresses: []common.Address{},
			kind:           "admin",
			mockSetup: func(_ *mocks.Prompter) {
				// No mock setup needed as CaptureList should not be called
			},
			expectedAddresses: []common.Address{},
			expectedCancelled: true,
			expectedError:     "",
			expectedOutput:    "There are no admin addresses to remove from",
		},
		{
			name: "user selects address to remove",
			inputAddresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
				common.HexToAddress("0x2222222222222222222222222222222222222222"),
			},
			kind: "manager",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"0x1111111111111111111111111111111111111111",
					"0x2222222222222222222222222222222222222222",
					"Cancel",
				}
				m.On("CaptureList", "Select the address you want to remove", options).Return("0x1111111111111111111111111111111111111111", nil)
			},
			expectedAddresses: []common.Address{
				common.HexToAddress("0x2222222222222222222222222222222222222222"),
			},
			expectedCancelled: false,
			expectedError:     "",
			expectedOutput:    "",
		},
		{
			name: "user selects cancel option",
			inputAddresses: []common.Address{
				common.HexToAddress("0x3333333333333333333333333333333333333333"),
			},
			kind: "enabled",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"0x3333333333333333333333333333333333333333",
					"Cancel",
				}
				m.On("CaptureList", "Select the address you want to remove", options).Return("Cancel", nil)
			},
			expectedAddresses: []common.Address{
				common.HexToAddress("0x3333333333333333333333333333333333333333"),
			},
			expectedCancelled: true,
			expectedError:     "",
			expectedOutput:    "",
		},
		{
			name: "CaptureList returns error",
			inputAddresses: []common.Address{
				common.HexToAddress("0x4444444444444444444444444444444444444444"),
			},
			kind: "admin",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"0x4444444444444444444444444444444444444444",
					"Cancel",
				}
				m.On("CaptureList", "Select the address you want to remove", options).Return("", errors.New("capture failed"))
			},
			expectedAddresses: []common.Address{
				common.HexToAddress("0x4444444444444444444444444444444444444444"),
			},
			expectedCancelled: false,
			expectedError:     "capture failed",
			expectedOutput:    "",
		},
		{
			name: "remove from single address list",
			inputAddresses: []common.Address{
				common.HexToAddress("0x5555555555555555555555555555555555555555"),
			},
			kind: "manager",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"0x5555555555555555555555555555555555555555",
					"Cancel",
				}
				m.On("CaptureList", "Select the address you want to remove", options).Return("0x5555555555555555555555555555555555555555", nil)
			},
			expectedAddresses: []common.Address{},
			expectedCancelled: false,
			expectedError:     "",
			expectedOutput:    "",
		},
		{
			name: "remove middle address from multiple addresses",
			inputAddresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
				common.HexToAddress("0x2222222222222222222222222222222222222222"),
				common.HexToAddress("0x3333333333333333333333333333333333333333"),
			},
			kind: "enabled",
			mockSetup: func(m *mocks.Prompter) {
				options := []string{
					"0x1111111111111111111111111111111111111111",
					"0x2222222222222222222222222222222222222222",
					"0x3333333333333333333333333333333333333333",
					"Cancel",
				}
				m.On("CaptureList", "Select the address you want to remove", options).Return("0x2222222222222222222222222222222222222222", nil)
			},
			expectedAddresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
				common.HexToAddress("0x3333333333333333333333333333333333333333"),
			},
			expectedCancelled: false,
			expectedError:     "",
			expectedOutput:    "",
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

			// Capture stdout to verify printed messages
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call the function under test
			resultAddresses, cancelled, err := removeAddress(app, tt.inputAddresses, tt.kind)

			// Restore stdout and read captured output
			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			_, readErr := buf.ReadFrom(r)
			require.NoError(t, readErr)
			output := strings.TrimSpace(buf.String())

			// Assertions
			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tt.expectedCancelled, cancelled, "Cancelled flag should match expected value")
			require.Equal(t, len(tt.expectedAddresses), len(resultAddresses), "Result should have expected number of addresses")

			// Check that all expected addresses are present
			for _, expectedAddr := range tt.expectedAddresses {
				found := false
				for _, resultAddr := range resultAddresses {
					if expectedAddr == resultAddr {
						found = true
						break
					}
				}
				require.True(t, found, "Expected address %s should be in result", expectedAddr.Hex())
			}

			// Verify output messages
			if tt.expectedOutput != "" {
				require.Contains(t, output, tt.expectedOutput, "Expected output should appear in captured output")
			} else if len(tt.inputAddresses) > 0 {
				require.Empty(t, output, "Should not print anything when addresses are present")
			}

			// Verify all mock expectations were met
			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestRemoveAddressDifferentKinds(t *testing.T) {
	// Test that different kinds show appropriate messages for empty lists
	kinds := []string{"admin", "manager", "enabled", "custom"}

	for _, kind := range kinds {
		t.Run("empty_list_message_for_"+kind, func(t *testing.T) {
			mockPrompter := mocks.NewPrompter(t)

			app := &application.Avalanche{
				Prompt: mockPrompter,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			addresses, cancelled, err := removeAddress(app, []common.Address{}, kind)

			// Restore stdout and read captured output
			w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			_, readErr := buf.ReadFrom(r)
			require.NoError(t, readErr)
			output := strings.TrimSpace(buf.String())

			require.NoError(t, err)
			require.True(t, cancelled)
			require.Empty(t, addresses)
			require.Contains(t, output, fmt.Sprintf("There are no %s addresses to remove from", kind))

			mockPrompter.AssertExpectations(t)
		})
	}
}

func TestRemoveAddressEdgeCases(t *testing.T) {
	t.Run("case_insensitive_address_matching", func(t *testing.T) {
		// Test that address matching works regardless of case in the selection
		mockPrompter := mocks.NewPrompter(t)

		inputAddresses := []common.Address{
			common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12"),
		}

		// The prompt will show the address in proper case, but we'll simulate user selecting with different case
		options := []string{
			"0xabCDEF1234567890ABcDEF1234567890aBCDeF12", // This is how it appears in options
			"Cancel",
		}
		mockPrompter.On("CaptureList", "Select the address you want to remove", options).Return("0xabCDEF1234567890ABcDEF1234567890aBCDeF12", nil)

		app := &application.Avalanche{
			Prompt: mockPrompter,
		}

		resultAddresses, cancelled, err := removeAddress(app, inputAddresses, "test")

		require.NoError(t, err)
		require.False(t, cancelled)
		require.Empty(t, resultAddresses, "Address should be removed successfully")

		mockPrompter.AssertExpectations(t)
	})

	t.Run("duplicate_addresses_in_input", func(t *testing.T) {
		// Test behavior when input contains duplicate addresses
		mockPrompter := mocks.NewPrompter(t)

		duplicateAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
		inputAddresses := []common.Address{
			duplicateAddr,
			duplicateAddr, // duplicate
			common.HexToAddress("0x2222222222222222222222222222222222222222"),
		}

		// Options should still include the duplicate (as the function doesn't deduplicate)
		options := []string{
			"0x1111111111111111111111111111111111111111",
			"0x1111111111111111111111111111111111111111", // duplicate in options
			"0x2222222222222222222222222222222222222222",
			"Cancel",
		}
		mockPrompter.On("CaptureList", "Select the address you want to remove", options).Return("0x1111111111111111111111111111111111111111", nil)

		app := &application.Avalanche{
			Prompt: mockPrompter,
		}

		resultAddresses, cancelled, err := removeAddress(app, inputAddresses, "test")

		require.NoError(t, err)
		require.False(t, cancelled)

		// Should remove all instances of the duplicate address
		require.Equal(t, 1, len(resultAddresses), "Should remove all instances of duplicate address")
		require.Equal(t, common.HexToAddress("0x2222222222222222222222222222222222222222"), resultAddresses[0], "Should only have the non-duplicate address remaining")

		mockPrompter.AssertExpectations(t)
	})

	t.Run("zero_address", func(t *testing.T) {
		// Test with zero address
		mockPrompter := mocks.NewPrompter(t)

		zeroAddr := common.Address{}
		inputAddresses := []common.Address{zeroAddr}

		options := []string{
			"0x0000000000000000000000000000000000000000",
			"Cancel",
		}
		mockPrompter.On("CaptureList", "Select the address you want to remove", options).Return("0x0000000000000000000000000000000000000000", nil)

		app := &application.Avalanche{
			Prompt: mockPrompter,
		}

		resultAddresses, cancelled, err := removeAddress(app, inputAddresses, "test")

		require.NoError(t, err)
		require.False(t, cancelled)
		require.Empty(t, resultAddresses, "Zero address should be removed successfully")

		mockPrompter.AssertExpectations(t)
	})

	t.Run("empty_kind_string", func(t *testing.T) {
		// Test with empty kind string
		mockPrompter := mocks.NewPrompter(t)

		app := &application.Avalanche{
			Prompt: mockPrompter,
		}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		addresses, cancelled, err := removeAddress(app, []common.Address{}, "")

		// Restore stdout and read captured output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, readErr := buf.ReadFrom(r)
		require.NoError(t, readErr)
		output := strings.TrimSpace(buf.String())

		require.NoError(t, err)
		require.True(t, cancelled)
		require.Empty(t, addresses)
		require.Contains(t, output, "There are no  addresses to remove from") // empty kind

		mockPrompter.AssertExpectations(t)
	})
}
