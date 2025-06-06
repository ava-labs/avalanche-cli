// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts/comparator"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/manifoldco/promptui"
	"github.com/stretchr/testify/require"
)

const (
	h720                      = "720h"
	invalidDuration           = "invalid duration"
	exceedsMaxStakingDuration = "exceeds maximum staking duration"
	belowMinStakingDuration   = "below the minimum staking duration"
)

func TestNewPrompter(t *testing.T) {
	t.Run("returns non-nil prompter", func(t *testing.T) {
		prompter := NewPrompter()
		require.NotNil(t, prompter)
	})

	t.Run("returns Prompter interface implementation", func(t *testing.T) {
		prompter := NewPrompter()
		// Verify it implements the Prompter interface by checking we can call interface methods
		require.NotNil(t, prompter)
	})

	t.Run("returns realPrompter instance", func(t *testing.T) {
		prompter := NewPrompter()

		// Type assertion to verify it's specifically a realPrompter
		realPrompter, ok := prompter.(*realPrompter)
		require.True(t, ok, "NewPrompter should return a *realPrompter")
		require.NotNil(t, realPrompter)
	})

	t.Run("creates new instance each time", func(t *testing.T) {
		prompter1 := NewPrompter()
		prompter2 := NewPrompter()

		require.NotNil(t, prompter1)
		require.NotNil(t, prompter2)

		// Verify they are different instances (different memory addresses)
		require.NotSame(t, prompter1, prompter2, "NewPrompter should create new instances each time")
	})

	t.Run("can call interface methods", func(t *testing.T) {
		// Save original function to avoid interfering with other tests
		originalRunner := promptUIRunner
		defer func() {
			promptUIRunner = originalRunner
		}()

		// Mock promptUIRunner to avoid actual user interaction
		promptUIRunner = func(promptui.Prompt) (string, error) {
			return "24h", nil
		}

		prompter := NewPrompter()

		// Test that we can actually call methods on the returned prompter
		duration, err := prompter.CaptureDuration("Test prompt")
		require.NoError(t, err)
		require.Equal(t, 24*time.Hour, duration)
	})
}

func TestPromptUIRunner(t *testing.T) {
	t.Run("function variable exists and is not nil", func(t *testing.T) {
		require.NotNil(t, promptUIRunner)
	})

	t.Run("can be replaced and restored", func(t *testing.T) {
		// Save original function
		originalRunner := promptUIRunner
		defer func() {
			promptUIRunner = originalRunner
		}()

		// Replace with a mock
		mockCalled := false
		promptUIRunner = func(promptui.Prompt) (string, error) {
			mockCalled = true
			return "mock result", nil
		}

		// Create a dummy prompt (we won't actually run it interactively)
		prompt := promptui.Prompt{
			Label: "Test prompt",
		}

		// Call the replaced function
		result, err := promptUIRunner(prompt)

		// Verify mock was called and returned expected values
		require.True(t, mockCalled)
		require.NoError(t, err)
		require.Equal(t, "mock result", result)

		// Verify we can restore the original (function comparison not possible in Go)
		promptUIRunner = originalRunner
		require.NotNil(t, promptUIRunner)
	})

	t.Run("mock can simulate errors", func(t *testing.T) {
		// Save original function
		originalRunner := promptUIRunner
		defer func() {
			promptUIRunner = originalRunner
		}()

		// Replace with a mock that returns an error
		expectedError := fmt.Errorf("mock error")
		promptUIRunner = func(promptui.Prompt) (string, error) {
			return "", expectedError
		}

		// Create a dummy prompt
		prompt := promptui.Prompt{
			Label: "Test prompt",
		}

		// Call the mocked function
		result, err := promptUIRunner(prompt)

		// Verify error was returned
		require.Error(t, err)
		require.Equal(t, expectedError, err)
		require.Empty(t, result)
	})

	t.Run("mock can access prompt properties", func(t *testing.T) {
		// Save original function
		originalRunner := promptUIRunner
		defer func() {
			promptUIRunner = originalRunner
		}()

		// Replace with a mock that inspects the prompt
		var receivedLabel string
		var receivedValidate func(string) error
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			if label, ok := prompt.Label.(string); ok {
				receivedLabel = label
			}
			receivedValidate = prompt.Validate
			return "inspected", nil
		}

		// Create a prompt with specific properties
		testValidator := func(string) error { return nil }
		prompt := promptui.Prompt{
			Label:    "Expected Label",
			Validate: testValidator,
		}

		// Call the mocked function
		result, err := promptUIRunner(prompt)

		// Verify the mock received the correct prompt properties
		require.NoError(t, err)
		require.Equal(t, "inspected", result)
		require.Equal(t, "Expected Label", receivedLabel)
		require.NotNil(t, receivedValidate)
	})

	t.Run("mock can be called multiple times with different results", func(t *testing.T) {
		// Save original function
		originalRunner := promptUIRunner
		defer func() {
			promptUIRunner = originalRunner
		}()

		// Replace with a mock that returns different results based on call count
		callCount := 0
		promptUIRunner = func(promptui.Prompt) (string, error) {
			callCount++
			switch callCount {
			case 1:
				return "first call", nil
			case 2:
				return "second call", nil
			default:
				return "subsequent call", nil
			}
		}

		prompt := promptui.Prompt{Label: "Test"}

		// First call
		result1, err1 := promptUIRunner(prompt)
		require.NoError(t, err1)
		require.Equal(t, "first call", result1)

		// Second call
		result2, err2 := promptUIRunner(prompt)
		require.NoError(t, err2)
		require.Equal(t, "second call", result2)

		// Third call
		result3, err3 := promptUIRunner(prompt)
		require.NoError(t, err3)
		require.Equal(t, "subsequent call", result3)

		require.Equal(t, 3, callCount)
	})

	t.Run("integration with actual prompter function", func(t *testing.T) {
		// Save original function
		originalRunner := promptUIRunner
		defer func() {
			promptUIRunner = originalRunner
		}()

		// Replace with a mock that simulates duration input
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Verify this is being called from CaptureDuration by checking the validator
			if prompt.Validate != nil {
				// Test with a valid duration to ensure it's the right validator
				err := prompt.Validate("24h")
				require.NoError(t, err, "Should be using duration validator")
			}
			return "48h", nil
		}

		// Create a prompter and call CaptureDuration
		prompter := &realPrompter{}
		duration, err := prompter.CaptureDuration("Test integration")

		// Verify the integration worked
		require.NoError(t, err)
		require.Equal(t, 48*time.Hour, duration)
	})
}

func TestCaptureDurationWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedDur   time.Duration
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid duration - hours",
			mockReturn:  "24h",
			mockError:   nil,
			expectedDur: 24 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - minutes",
			mockReturn:  "30m",
			mockError:   nil,
			expectedDur: 30 * time.Minute,
			expectError: false,
		},
		{
			name:        "valid duration - seconds",
			mockReturn:  "45s",
			mockError:   nil,
			expectedDur: 45 * time.Second,
			expectError: false,
		},
		{
			name:        "valid duration - complex",
			mockReturn:  "1h30m",
			mockError:   nil,
			expectedDur: time.Hour + 30*time.Minute,
			expectError: false,
		},
		{
			name:          "invalid duration format",
			mockReturn:    "invalid",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: invalidDuration,
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedDur:   0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "prompt error - validation failed",
			mockReturn:    "",
			mockError:     fmt.Errorf("validation failed"),
			expectedDur:   0,
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name:        "zero duration",
			mockReturn:  "0s",
			mockError:   nil,
			expectedDur: 0,
			expectError: false,
		},
		{
			name:        "negative duration",
			mockReturn:  "-1h",
			mockError:   nil,
			expectedDur: -time.Hour,
			expectError: false,
		},
		{
			name:        "fractional duration",
			mockReturn:  "1.5h",
			mockError:   nil,
			expectedDur: time.Hour + 30*time.Minute,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter duration:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				// Test that the validator works as expected
				if tt.mockReturn != "" && tt.mockError == nil {
					err := prompt.Validate(tt.mockReturn)
					if tt.errorContains == invalidDuration {
						require.Error(t, err) // Should fail validation
					} else {
						require.NoError(t, err) // Should pass validation
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			duration, err := prompter.CaptureDuration("Enter duration:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, time.Duration(0), duration)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedDur, duration)
			}
		})
	}
}

func TestCaptureDurationEdgeCases(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	t.Run("validation function called correctly", func(t *testing.T) {
		validationCalled := false
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Call the validation function to ensure it's the right one
			err := prompt.Validate("24h")
			require.NoError(t, err)
			validationCalled = true
			return "24h", nil
		}

		prompter := &realPrompter{}
		duration, err := prompter.CaptureDuration("Test prompt")

		require.NoError(t, err)
		require.Equal(t, 24*time.Hour, duration)
		require.True(t, validationCalled)
	})

	t.Run("prompt label preserved", func(t *testing.T) {
		expectedLabel := "Please enter the duration for staking"
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			require.Equal(t, expectedLabel, prompt.Label)
			return "12h", nil
		}

		prompter := &realPrompter{}
		duration, err := prompter.CaptureDuration(expectedLabel)

		require.NoError(t, err)
		require.Equal(t, 12*time.Hour, duration)
	})

	t.Run("time.ParseDuration error handling", func(t *testing.T) {
		// Mock the promptUIRunner to return a string that passes validation
		// but fails time.ParseDuration (this is hypothetical since validateDuration
		// should catch invalid formats, but tests the error path)
		promptUIRunner = func(promptui.Prompt) (string, error) {
			// Return a valid duration that should pass validation
			return "1h", nil
		}

		prompter := &realPrompter{}
		duration, err := prompter.CaptureDuration("Enter duration:")

		require.NoError(t, err)
		require.Equal(t, time.Hour, duration)
	})
}

func TestCaptureFujiDurationWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedDur   time.Duration
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid duration - within Fuji range",
			mockReturn:  h720, // 30 days, should be within Fuji range
			mockError:   nil,
			expectedDur: 720 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - minimum (24h)",
			mockReturn:  "24h", // 1 day, should be minimum for Fuji
			mockError:   nil,
			expectedDur: 24 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - several days",
			mockReturn:  "168h", // 7 days
			mockError:   nil,
			expectedDur: 168 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - complex format (2 days)",
			mockReturn:  "48h30m",
			mockError:   nil,
			expectedDur: 48*time.Hour + 30*time.Minute,
			expectError: false,
		},
		{
			name:          "invalid duration - too short for Fuji (30 minutes)",
			mockReturn:    "30m",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
		{
			name:          "invalid duration - too short for Fuji (1 hour)",
			mockReturn:    "1h",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
		{
			name:          "invalid duration - too long for Fuji",
			mockReturn:    "9600h", // 400 days, should exceed Fuji max
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: exceedsMaxStakingDuration,
		},
		{
			name:          "invalid duration format",
			mockReturn:    "invalid",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: invalidDuration,
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedDur:   0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "prompt error - validation failed",
			mockReturn:    "",
			mockError:     fmt.Errorf("validation failed"),
			expectedDur:   0,
			expectError:   true,
			errorContains: "validation failed",
		},
		{
			name:          "invalid duration - zero duration",
			mockReturn:    "0s",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
		{
			name:          "invalid duration - negative duration",
			mockReturn:    "-1h",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter Fuji staking duration:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				// If we expect a validation error, simulate the prompt validation failing
				if tt.mockReturn != "" && tt.mockError == nil {
					if tt.errorContains == belowMinStakingDuration ||
						tt.errorContains == exceedsMaxStakingDuration ||
						tt.errorContains == invalidDuration {
						// Test that validation actually fails
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err) // Should fail validation
						// Return the validation error (simulating what promptui would do)
						return "", err
					} else {
						// Test that validation passes
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err) // Should pass validation
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			duration, err := prompter.CaptureFujiDuration("Enter Fuji staking duration:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, time.Duration(0), duration)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedDur, duration)
			}
		})
	}
}

func TestCaptureFujiDurationEdgeCases(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	t.Run("validation function called correctly", func(t *testing.T) {
		validationCalled := false
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Call the validation function to ensure it's the right one
			err := prompt.Validate(h720) // Should be valid for Fuji
			require.NoError(t, err)
			validationCalled = true
			return h720, nil
		}

		prompter := &realPrompter{}
		duration, err := prompter.CaptureFujiDuration("Test Fuji prompt")

		require.NoError(t, err)
		require.Equal(t, 720*time.Hour, duration)
		require.True(t, validationCalled)
	})

	t.Run("prompt label preserved", func(t *testing.T) {
		expectedLabel := "Please enter the duration for Fuji staking"
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			require.Equal(t, expectedLabel, prompt.Label)
			return h720, nil
		}

		prompter := &realPrompter{}
		duration, err := prompter.CaptureFujiDuration(expectedLabel)

		require.NoError(t, err)
		require.Equal(t, 720*time.Hour, duration)
	})

	t.Run("Fuji-specific validation", func(t *testing.T) {
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Test that it uses Fuji-specific validation (different from general duration validation)
			err1 := prompt.Validate("1h") // Should fail for Fuji (too short)
			require.Error(t, err1)
			require.Contains(t, err1.Error(), belowMinStakingDuration)

			err2 := prompt.Validate("9600h") // Should fail for Fuji (too long)
			require.Error(t, err2)
			require.Contains(t, err2.Error(), exceedsMaxStakingDuration)

			return h720, nil // Return valid duration
		}

		prompter := &realPrompter{}
		duration, err := prompter.CaptureFujiDuration("Enter Fuji duration:")

		require.NoError(t, err)
		require.Equal(t, 720*time.Hour, duration)
	})
}

func TestCaptureMainnetDurationWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedDur   time.Duration
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid duration - within Mainnet range",
			mockReturn:  "8760h", // 365 days, should be within Mainnet range
			mockError:   nil,
			expectedDur: 8760 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - several weeks",
			mockReturn:  "336h", // 14 days
			mockError:   nil,
			expectedDur: 336 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - complex format",
			mockReturn:  "720h30m", // 30 days and 30 minutes
			mockError:   nil,
			expectedDur: 720*time.Hour + 30*time.Minute,
			expectError: false,
		},
		{
			name:          "invalid duration - too short for Mainnet",
			mockReturn:    "1h",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
		{
			name:          "invalid duration - too long for Mainnet",
			mockReturn:    "9000h", // Should exceed Mainnet max (around 365 days)
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: exceedsMaxStakingDuration,
		},
		{
			name:          "invalid duration format",
			mockReturn:    "invalid",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: invalidDuration,
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedDur:   0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "invalid duration - zero duration",
			mockReturn:    "0s",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter Mainnet staking duration:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				// If we expect a validation error, simulate the prompt validation failing
				if tt.mockReturn != "" && tt.mockError == nil {
					if tt.errorContains == belowMinStakingDuration ||
						tt.errorContains == exceedsMaxStakingDuration ||
						tt.errorContains == invalidDuration {
						// Test that validation actually fails
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err) // Should fail validation
						// Return the validation error (simulating what promptui would do)
						return "", err
					} else {
						// Test that validation passes
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err) // Should pass validation
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			duration, err := prompter.CaptureMainnetDuration("Enter Mainnet staking duration:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, time.Duration(0), duration)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedDur, duration)
			}
		})
	}
}

func TestCaptureMainnetL1StakingDurationWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedDur   time.Duration
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid duration - within Mainnet L1 range",
			mockReturn:  h720, // 30 days, should be within range
			mockError:   nil,
			expectedDur: 720 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - minimum (24h)",
			mockReturn:  "24h", // Minimum for L1 staking
			mockError:   nil,
			expectedDur: 24 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - several weeks",
			mockReturn:  "168h", // 7 days
			mockError:   nil,
			expectedDur: 168 * time.Hour,
			expectError: false,
		},
		{
			name:        "valid duration - complex format",
			mockReturn:  "48h30m", // 2 days and 30 minutes
			mockError:   nil,
			expectedDur: 48*time.Hour + 30*time.Minute,
			expectError: false,
		},
		{
			name:          "invalid duration - too short for L1 (12 hours)",
			mockReturn:    "12h",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
		{
			name:          "invalid duration - too short for L1 (1 hour)",
			mockReturn:    "1h",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
		{
			name:          "invalid duration - too long for Mainnet L1",
			mockReturn:    "9000h", // Should exceed Mainnet max
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: exceedsMaxStakingDuration,
		},
		{
			name:          "invalid duration format",
			mockReturn:    "invalid",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: invalidDuration,
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedDur:   0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "invalid duration - zero duration",
			mockReturn:    "0s",
			mockError:     nil,
			expectedDur:   0,
			expectError:   true,
			errorContains: belowMinStakingDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter Mainnet L1 staking duration:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				// If we expect a validation error, simulate the prompt validation failing
				if tt.mockReturn != "" && tt.mockError == nil {
					if tt.errorContains == belowMinStakingDuration ||
						tt.errorContains == exceedsMaxStakingDuration ||
						tt.errorContains == invalidDuration {
						// Test that validation actually fails
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err) // Should fail validation
						// Return the validation error (simulating what promptui would do)
						return "", err
					} else {
						// Test that validation passes
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err) // Should pass validation
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			duration, err := prompter.CaptureMainnetL1StakingDuration("Enter Mainnet L1 staking duration:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, time.Duration(0), duration)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedDur, duration)
			}
		})
	}
}

func TestCaptureDateWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	// Create test times with sufficient buffer for processing delays
	now := time.Now().UTC()
	futureTime := now.Add(time.Hour) // Well beyond the 5-minute lead time
	pastTime := now.Add(-time.Hour)
	closeTime := now.Add(4 * time.Minute) // Less than 5-minute lead time

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedTime  time.Time
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid future time",
			mockReturn:   futureTime.Format("2006-01-02 15:04:05"),
			mockError:    nil,
			expectedTime: futureTime.Truncate(time.Second), // Truncate to seconds precision
			expectError:  false,
		},
		{
			name:         "valid time with exact format",
			mockReturn:   "2025-12-25 15:30:45",
			mockError:    nil,
			expectedTime: time.Date(2025, 12, 25, 15, 30, 45, 0, time.UTC),
			expectError:  false,
		},
		{
			name:          "invalid time - too close to now",
			mockReturn:    closeTime.Format("2006-01-02 15:04:05"),
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "time should be at least start from now",
		},
		{
			name:          "invalid time - in the past",
			mockReturn:    pastTime.Format("2006-01-02 15:04:05"),
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "time should be at least start from now",
		},
		{
			name:          "invalid time format - wrong layout",
			mockReturn:    "2025-12-25T15:30:45Z",
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "parsing time",
		},
		{
			name:          "invalid time format - missing seconds",
			mockReturn:    "2025-12-25 15:30",
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "parsing time",
		},
		{
			name:          "invalid time format - random string",
			mockReturn:    "invalid-time",
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "parsing time",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "prompt error - validation failed",
			mockReturn:    "",
			mockError:     fmt.Errorf("validation failed"),
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter date:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				// If we expect a validation error, simulate the prompt validation failing
				if tt.mockReturn != "" && tt.mockError == nil {
					if tt.errorContains == "time should be at least start from now" ||
						tt.errorContains == "parsing time" {
						// Test that validation actually fails
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err) // Should fail validation
						// Return the validation error (simulating what promptui would do)
						return "", err
					} else {
						// Test that validation passes
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err) // Should pass validation
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			capturedTime, err := prompter.CaptureDate("Enter date:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, time.Time{}, capturedTime)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedTime, capturedTime)
			}
		})
	}
}

func TestCaptureIDWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	// Generate test IDs
	validID := ids.GenerateTestID()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedID    ids.ID
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid ID",
			mockReturn:  validID.String(),
			mockError:   nil,
			expectedID:  validID,
			expectError: false,
		},
		{
			name:          "invalid ID format - too short",
			mockReturn:    "invalid",
			mockError:     nil,
			expectedID:    ids.Empty,
			expectError:   true,
			errorContains: "base58 decoding error",
		},
		{
			name:          "invalid ID format - wrong format",
			mockReturn:    "invalidID123",
			mockError:     nil,
			expectedID:    ids.Empty,
			expectError:   true,
			errorContains: "base58 decoding error",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedID:    ids.Empty,
			expectError:   true,
			errorContains: "zero length string",
		},
		{
			name:          "invalid characters",
			mockReturn:    "2Z4UuXuKg@#$%^&*()",
			mockError:     nil,
			expectedID:    ids.Empty,
			expectError:   true,
			errorContains: "Invalid base58 digit",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedID:    ids.Empty,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "prompt error - validation failed",
			mockReturn:    "",
			mockError:     fmt.Errorf("validation failed"),
			expectedID:    ids.Empty,
			expectError:   true,
			errorContains: "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter ID:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				// If we expect a validation error, test the validation
				if tt.mockReturn != "" && tt.mockError == nil {
					if tt.errorContains == "base58 decoding error" ||
						tt.errorContains == "zero length string" ||
						tt.errorContains == "Invalid base58 digit" {
						// Test that validation actually fails
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err) // Should fail validation
						// Return the validation error (simulating what promptui would do)
						return "", err
					} else {
						// Test that validation passes
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err) // Should pass validation
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			id, err := prompter.CaptureID("Enter ID:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, ids.Empty, id)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedID, id)
			}
		})
	}
}

func TestCaptureIDEdgeCases(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	t.Run("validation function called correctly", func(t *testing.T) {
		validID := ids.GenerateTestID()
		validationCalled := false
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Call the validation function to ensure it's the right one
			err := prompt.Validate(validID.String())
			require.NoError(t, err)
			validationCalled = true
			return validID.String(), nil
		}

		prompter := &realPrompter{}
		id, err := prompter.CaptureID("Test ID prompt")

		require.NoError(t, err)
		require.Equal(t, validID, id)
		require.True(t, validationCalled)
	})

	t.Run("prompt label preserved", func(t *testing.T) {
		validID := ids.GenerateTestID()
		expectedLabel := "Please enter the validation ID"
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			require.Equal(t, expectedLabel, prompt.Label)
			return validID.String(), nil
		}

		prompter := &realPrompter{}
		id, err := prompter.CaptureID(expectedLabel)

		require.NoError(t, err)
		require.Equal(t, validID, id)
	})

	t.Run("ID validation", func(t *testing.T) {
		validID := ids.GenerateTestID()
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Test various invalid IDs
			err1 := prompt.Validate("invalid")
			require.Error(t, err1)

			err2 := prompt.Validate("")
			require.Error(t, err2)

			err3 := prompt.Validate("2Z4UuXuKg@#$%^&*()")
			require.Error(t, err3)

			// Test valid ID
			err4 := prompt.Validate(validID.String())
			require.NoError(t, err4)

			return validID.String(), nil
		}

		prompter := &realPrompter{}
		id, err := prompter.CaptureID("Enter ID:")

		require.NoError(t, err)
		require.Equal(t, validID, id)
	})
}

func TestCaptureNodeIDWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	// Generate test Node IDs
	validNodeID := ids.GenerateTestNodeID()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedID    ids.NodeID
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid node ID",
			mockReturn:  validNodeID.String(),
			mockError:   nil,
			expectedID:  validNodeID,
			expectError: false,
		},
		{
			name:          "invalid node ID - too short",
			mockReturn:    "invalid",
			mockError:     nil,
			expectedID:    ids.EmptyNodeID,
			expectError:   true,
			errorContains: "missing the prefix: NodeID-",
		},
		{
			name:          "invalid node ID - wrong format",
			mockReturn:    "NodeID-InvalidFormat123",
			mockError:     nil,
			expectedID:    ids.EmptyNodeID,
			expectError:   true,
			errorContains: "base58 decoding error",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedID:    ids.EmptyNodeID,
			expectError:   true,
			errorContains: "missing the prefix: NodeID-",
		},
		{
			name:          "invalid characters",
			mockReturn:    "NodeID-@#$%^&*()",
			mockError:     nil,
			expectedID:    ids.EmptyNodeID,
			expectError:   true,
			errorContains: "Invalid base58 digit",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedID:    ids.EmptyNodeID,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "prompt error - validation failed",
			mockReturn:    "",
			mockError:     fmt.Errorf("validation failed"),
			expectedID:    ids.EmptyNodeID,
			expectError:   true,
			errorContains: "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter Node ID:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				// If we expect a validation error, test the validation
				if tt.mockReturn != "" && tt.mockError == nil {
					if tt.errorContains == "missing the prefix: NodeID-" ||
						tt.errorContains == "base58 decoding error" ||
						tt.errorContains == "Invalid base58 digit" {
						// Test that validation actually fails
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err) // Should fail validation
						// Return the validation error (simulating what promptui would do)
						return "", err
					} else {
						// Test that validation passes
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err) // Should pass validation
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			nodeID, err := prompter.CaptureNodeID("Enter Node ID:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, ids.EmptyNodeID, nodeID)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedID, nodeID)
			}
		})
	}
}

func TestCaptureNodeIDEdgeCases(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	t.Run("validation function called correctly", func(t *testing.T) {
		validNodeID := ids.GenerateTestNodeID()
		validationCalled := false
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Call the validation function to ensure it's the right one
			err := prompt.Validate(validNodeID.String())
			require.NoError(t, err)
			validationCalled = true
			return validNodeID.String(), nil
		}

		prompter := &realPrompter{}
		nodeID, err := prompter.CaptureNodeID("Test Node ID prompt")

		require.NoError(t, err)
		require.Equal(t, validNodeID, nodeID)
		require.True(t, validationCalled)
	})

	t.Run("prompt label preserved", func(t *testing.T) {
		validNodeID := ids.GenerateTestNodeID()
		expectedLabel := "Please enter the node ID"
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			require.Equal(t, expectedLabel, prompt.Label)
			return validNodeID.String(), nil
		}

		prompter := &realPrompter{}
		nodeID, err := prompter.CaptureNodeID(expectedLabel)

		require.NoError(t, err)
		require.Equal(t, validNodeID, nodeID)
	})

	t.Run("Node ID validation", func(t *testing.T) {
		validNodeID := ids.GenerateTestNodeID()
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Test various invalid Node IDs
			err1 := prompt.Validate("invalid")
			require.Error(t, err1)

			err2 := prompt.Validate("")
			require.Error(t, err2)

			err3 := prompt.Validate("NodeID-InvalidFormat123")
			require.Error(t, err3)

			err4 := prompt.Validate("NodeID-@#$%^&*()")
			require.Error(t, err4)

			// Test valid Node ID
			err5 := prompt.Validate(validNodeID.String())
			require.NoError(t, err5)

			return validNodeID.String(), nil
		}

		prompter := &realPrompter{}
		nodeID, err := prompter.CaptureNodeID("Enter Node ID:")

		require.NoError(t, err)
		require.Equal(t, validNodeID, nodeID)
	})
}

func TestCaptureValidatorBalanceWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name             string
		availableBalance float64
		minBalance       float64
		mockReturn       string
		mockError        error
		expectedBalance  float64
		expectError      bool
		errorContains    string
	}{
		{
			name:             "valid balance within range",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "50.0",
			mockError:        nil,
			expectedBalance:  50.0,
			expectError:      false,
		},
		{
			name:             "minimum valid balance",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "10.0",
			mockError:        nil,
			expectedBalance:  10.0,
			expectError:      false,
		},
		{
			name:             "maximum available balance",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "1000.0",
			mockError:        nil,
			expectedBalance:  1000.0,
			expectError:      false,
		},
		{
			name:             "decimal balance",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "25.75",
			mockError:        nil,
			expectedBalance:  25.75,
			expectError:      false,
		},
		{
			name:             "zero balance",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "0",
			mockError:        nil,
			expectedBalance:  0,
			expectError:      true,
			errorContains:    "entered value has to be greater than 0 AVAX",
		},
		{
			name:             "balance below minimum",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "5.0",
			mockError:        nil,
			expectedBalance:  0,
			expectError:      true,
			errorContains:    "validator balance must be at least 10.00 AVAX",
		},
		{
			name:             "balance above available",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "1500.0",
			mockError:        nil,
			expectedBalance:  0,
			expectError:      true,
			errorContains:    "current balance of 1000.00 is not sufficient",
		},
		{
			name:             "invalid format - not a number",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "abc",
			mockError:        nil,
			expectedBalance:  0,
			expectError:      true,
			errorContains:    "invalid syntax",
		},
		{
			name:             "empty string",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "",
			mockError:        nil,
			expectedBalance:  0,
			expectError:      true,
			errorContains:    "invalid syntax",
		},
		{
			name:             "prompt error - user cancelled",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "",
			mockError:        fmt.Errorf("user cancelled"),
			expectedBalance:  0,
			expectError:      true,
			errorContains:    "user cancelled",
		},
		{
			name:             "prompt error - validation failed",
			availableBalance: 1000.0,
			minBalance:       10.0,
			mockReturn:       "",
			mockError:        fmt.Errorf("validation failed"),
			expectedBalance:  0,
			expectError:      true,
			errorContains:    "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter validator balance:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				// If we expect a validation error, simulate user cancellation
				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "entered value has to be greater than 0 AVAX") ||
						strings.Contains(tt.errorContains, "validator balance must be at least") ||
						strings.Contains(tt.errorContains, "current balance of"):
						// Test that validation actually fails
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			balance, err := prompter.CaptureValidatorBalance("Enter validator balance:", tt.availableBalance, tt.minBalance)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, 0.0, balance)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedBalance, balance)
			}
		})
	}
}

func TestCaptureValidatorBalanceEdgeCases(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	t.Run("validation function called correctly", func(t *testing.T) {
		validationCalled := false
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Call the validation function to ensure it's the right one
			err := prompt.Validate("50.0")
			require.NoError(t, err)
			validationCalled = true
			return "50.0", nil
		}

		prompter := &realPrompter{}
		balance, err := prompter.CaptureValidatorBalance("Test balance prompt", 1000.0, 10.0)

		require.NoError(t, err)
		require.Equal(t, 50.0, balance)
		require.True(t, validationCalled)
	})

	t.Run("prompt label preserved", func(t *testing.T) {
		expectedLabel := "Please enter the validator balance in AVAX"
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			require.Equal(t, expectedLabel, prompt.Label)
			return "25.0", nil
		}

		prompter := &realPrompter{}
		balance, err := prompter.CaptureValidatorBalance(expectedLabel, 1000.0, 10.0)

		require.NoError(t, err)
		require.Equal(t, 25.0, balance)
	})

	t.Run("validator balance validation logic", func(t *testing.T) {
		availableBalance := 100.0
		minBalance := 5.0
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Test various validation scenarios
			err1 := prompt.Validate("0") // Zero balance
			require.Error(t, err1)
			require.Contains(t, err1.Error(), "entered value has to be greater than 0 AVAX")

			err2 := prompt.Validate("3.0") // Below minimum
			require.Error(t, err2)
			require.Contains(t, err2.Error(), "validator balance must be at least 5.00 AVAX")

			err3 := prompt.Validate("150.0") // Above available
			require.Error(t, err3)
			require.Contains(t, err3.Error(), "current balance of 100.00 is not sufficient")

			err4 := prompt.Validate("abc") // Invalid format
			require.Error(t, err4)

			// Test valid balance
			err5 := prompt.Validate("50.0")
			require.NoError(t, err5)

			return "50.0", nil
		}

		prompter := &realPrompter{}
		balance, err := prompter.CaptureValidatorBalance("Enter balance:", availableBalance, minBalance)

		require.NoError(t, err)
		require.Equal(t, 50.0, balance)
	})

	t.Run("edge case - balance at boundaries", func(t *testing.T) {
		availableBalance := 100.0
		minBalance := 10.0
		promptUIRunner = func(prompt promptui.Prompt) (string, error) {
			// Test exact minimum
			err1 := prompt.Validate("10.0")
			require.NoError(t, err1)

			// Test exact maximum
			err2 := prompt.Validate("100.0")
			require.NoError(t, err2)

			// Test just below minimum
			err3 := prompt.Validate("9.99")
			require.Error(t, err3)

			// Test just above maximum
			err4 := prompt.Validate("100.01")
			require.Error(t, err4)

			return "10.0", nil
		}

		prompter := &realPrompter{}
		balance, err := prompter.CaptureValidatorBalance("Enter balance:", availableBalance, minBalance)

		require.NoError(t, err)
		require.Equal(t, 10.0, balance)
	})
}

func TestCaptureWeightWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name           string
		mockReturn     string
		mockError      error
		validator      func(uint64) error
		expectedWeight uint64
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid weight without extra validator",
			mockReturn:     "50",
			mockError:      nil,
			validator:      nil,
			expectedWeight: 50,
			expectError:    false,
		},
		{
			name:           "minimum weight",
			mockReturn:     "1",
			mockError:      nil,
			validator:      nil,
			expectedWeight: 1,
			expectError:    false,
		},
		{
			name:           "large valid weight",
			mockReturn:     "100",
			mockError:      nil,
			validator:      nil,
			expectedWeight: 100,
			expectError:    false,
		},
		{
			name:           "valid weight with passing validator",
			mockReturn:     "25",
			mockError:      nil,
			validator:      func(uint64) error { return nil },
			expectedWeight: 25,
			expectError:    false,
		},
		{
			name:           "zero weight (below minimum)",
			mockReturn:     "0",
			mockError:      nil,
			validator:      nil,
			expectedWeight: 0,
			expectError:    true,
			errorContains:  "the weight must be an integer between 1 and 100",
		},
		{
			name:           "weight with failing extra validator",
			mockReturn:     "50",
			mockError:      nil,
			validator:      func(uint64) error { return fmt.Errorf("custom validation failed") },
			expectedWeight: 0,
			expectError:    true,
			errorContains:  "custom validation failed",
		},
		{
			name:           "invalid format - not a number",
			mockReturn:     "abc",
			mockError:      nil,
			validator:      nil,
			expectedWeight: 0,
			expectError:    true,
			errorContains:  "invalid syntax",
		},
		{
			name:           "empty string",
			mockReturn:     "",
			mockError:      nil,
			validator:      nil,
			expectedWeight: 0,
			expectError:    true,
			errorContains:  "invalid syntax",
		},
		{
			name:           "prompt error - user cancelled",
			mockReturn:     "",
			mockError:      fmt.Errorf("user cancelled"),
			validator:      nil,
			expectedWeight: 0,
			expectError:    true,
			errorContains:  "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, "Enter weight:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "the weight must be an integer between 1 and 100"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "custom validation failed"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			weight, err := prompter.CaptureWeight("Enter weight:", tt.validator)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, uint64(0), weight)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedWeight, weight)
			}
		})
	}
}

func TestCaptureIntWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		validator     func(int) error
		expectedInt   int
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid positive integer",
			mockReturn:  "42",
			mockError:   nil,
			validator:   func(int) error { return nil },
			expectedInt: 42,
			expectError: false,
		},
		{
			name:        "valid negative integer",
			mockReturn:  "-10",
			mockError:   nil,
			validator:   func(int) error { return nil },
			expectedInt: -10,
			expectError: false,
		},
		{
			name:        "zero integer",
			mockReturn:  "0",
			mockError:   nil,
			validator:   func(int) error { return nil },
			expectedInt: 0,
			expectError: false,
		},
		{
			name:        "large integer",
			mockReturn:  "2147483647",
			mockError:   nil,
			validator:   func(int) error { return nil },
			expectedInt: 2147483647,
			expectError: false,
		},
		{
			name:          "integer with failing validator",
			mockReturn:    "100",
			mockError:     nil,
			validator:     func(val int) error { return fmt.Errorf("value %d not allowed", val) },
			expectedInt:   0,
			expectError:   true,
			errorContains: "value 100 not allowed",
		},
		{
			name:          "invalid format - not a number",
			mockReturn:    "abc",
			mockError:     nil,
			validator:     func(int) error { return nil },
			expectedInt:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - float",
			mockReturn:    "42.5",
			mockError:     nil,
			validator:     func(int) error { return nil },
			expectedInt:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			validator:     func(int) error { return nil },
			expectedInt:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			validator:     func(int) error { return nil },
			expectedInt:   0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "validation parsing failure - strconv.Atoi fails in Validate",
			mockReturn:    "not-a-number",
			mockError:     nil,
			validator:     func(int) error { return nil },
			expectedInt:   0,
			expectError:   true,
			errorContains: "strconv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, "Enter integer:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "not allowed"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "strconv"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			intVal, err := prompter.CaptureInt("Enter integer:", tt.validator)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, 0, intVal)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedInt, intVal)
			}
		})
	}
}

func TestCaptureUint8WithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedUint  uint8
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid uint8 - minimum",
			mockReturn:   "0",
			mockError:    nil,
			expectedUint: 0,
			expectError:  false,
		},
		{
			name:         "valid uint8 - maximum",
			mockReturn:   "255",
			mockError:    nil,
			expectedUint: 255,
			expectError:  false,
		},
		{
			name:         "valid uint8 - middle value",
			mockReturn:   "128",
			mockError:    nil,
			expectedUint: 128,
			expectError:  false,
		},
		{
			name:         "valid hex number",
			mockReturn:   "0xFF",
			mockError:    nil,
			expectedUint: 255,
			expectError:  false,
		},
		{
			name:         "valid octal number",
			mockReturn:   "0377",
			mockError:    nil,
			expectedUint: 255,
			expectError:  false,
		},
		{
			name:          "invalid - exceeds uint8 max",
			mockReturn:    "256",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "value out of range",
		},
		{
			name:          "invalid - negative number",
			mockReturn:    "-1",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - not a number",
			mockReturn:    "abc",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - float",
			mockReturn:    "42.5",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedUint:  0,
			expectError:   true,
			errorContains: "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, "Enter uint8:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "value out of range") ||
						strings.Contains(tt.errorContains, "invalid syntax"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			uint8Val, err := prompter.CaptureUint8("Enter uint8:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, uint8(0), uint8Val)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedUint, uint8Val)
			}
		})
	}
}

func TestCaptureUint16WithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedUint  uint16
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid uint16 - minimum",
			mockReturn:   "0",
			mockError:    nil,
			expectedUint: 0,
			expectError:  false,
		},
		{
			name:         "valid uint16 - maximum",
			mockReturn:   "65535",
			mockError:    nil,
			expectedUint: 65535,
			expectError:  false,
		},
		{
			name:         "valid uint16 - middle value",
			mockReturn:   "32768",
			mockError:    nil,
			expectedUint: 32768,
			expectError:  false,
		},
		{
			name:         "valid hex number",
			mockReturn:   "0xFFFF",
			mockError:    nil,
			expectedUint: 65535,
			expectError:  false,
		},
		{
			name:          "invalid - exceeds uint16 max",
			mockReturn:    "65536",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "value out of range",
		},
		{
			name:          "invalid - negative number",
			mockReturn:    "-1",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - not a number",
			mockReturn:    "abc",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedUint:  0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "validation parsing failure - strconv.ParseUint fails in Validate",
			mockReturn:    "not-a-number",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "strconv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, "Enter uint16:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "strconv"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "value out of range") ||
						strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			uint16Val, err := prompter.CaptureUint16("Enter uint16:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, uint16(0), uint16Val)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedUint, uint16Val)
			}
		})
	}
}

func TestCaptureUint32WithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedUint  uint32
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid uint32 - minimum",
			mockReturn:   "0",
			mockError:    nil,
			expectedUint: 0,
			expectError:  false,
		},
		{
			name:         "valid uint32 - maximum",
			mockReturn:   "4294967295",
			mockError:    nil,
			expectedUint: 4294967295,
			expectError:  false,
		},
		{
			name:         "valid uint32 - middle value",
			mockReturn:   "2147483648",
			mockError:    nil,
			expectedUint: 2147483648,
			expectError:  false,
		},
		{
			name:         "valid hex number",
			mockReturn:   "0xFFFFFFFF",
			mockError:    nil,
			expectedUint: 4294967295,
			expectError:  false,
		},
		{
			name:          "invalid - exceeds uint32 max",
			mockReturn:    "4294967296",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "value out of range",
		},
		{
			name:          "invalid - negative number",
			mockReturn:    "-1",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - not a number",
			mockReturn:    "abc",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedUint:  0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "validation parsing failure - strconv.ParseUint fails in Validate",
			mockReturn:    "not-a-number",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "strconv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, "Enter uint32:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "strconv"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "value out of range") ||
						strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			uint32Val, err := prompter.CaptureUint32("Enter uint32:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, uint32(0), uint32Val)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedUint, uint32Val)
			}
		})
	}
}

func TestCaptureUint64WithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedUint  uint64
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid uint64 - minimum positive",
			mockReturn:   "1",
			mockError:    nil,
			expectedUint: 1,
			expectError:  false,
		},
		{
			name:         "valid uint64 - large number",
			mockReturn:   "18446744073709551615",
			mockError:    nil,
			expectedUint: 18446744073709551615,
			expectError:  false,
		},
		{
			name:         "valid uint64 - middle value",
			mockReturn:   "9223372036854775808",
			mockError:    nil,
			expectedUint: 9223372036854775808,
			expectError:  false,
		},
		{
			name:         "valid hex number",
			mockReturn:   "0xFF",
			mockError:    nil,
			expectedUint: 255,
			expectError:  false,
		},
		{
			name:         "valid octal number",
			mockReturn:   "0755",
			mockError:    nil,
			expectedUint: 493,
			expectError:  false,
		},
		{
			name:          "invalid - zero (not bigger than zero)",
			mockReturn:    "0",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "the value must be bigger than zero",
		},
		{
			name:          "invalid - negative number",
			mockReturn:    "-1",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - not a number",
			mockReturn:    "abc",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - float",
			mockReturn:    "42.5",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedUint:  0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedUint:  0,
			expectError:   true,
			errorContains: "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, "Enter uint64:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "the value must be bigger than zero"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			uint64Val, err := prompter.CaptureUint64("Enter uint64:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, uint64(0), uint64Val)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedUint, uint64Val)
			}
		})
	}
}

func TestCaptureFloatWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		validator     func(float64) error
		expectedFloat float64
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid positive float",
			mockReturn:    "42.5",
			mockError:     nil,
			validator:     func(float64) error { return nil },
			expectedFloat: 42.5,
			expectError:   false,
		},
		{
			name:          "valid negative float",
			mockReturn:    "-10.75",
			mockError:     nil,
			validator:     func(float64) error { return nil },
			expectedFloat: -10.75,
			expectError:   false,
		},
		{
			name:          "valid zero float",
			mockReturn:    "0.0",
			mockError:     nil,
			validator:     func(float64) error { return nil },
			expectedFloat: 0.0,
			expectError:   false,
		},
		{
			name:          "valid integer as float",
			mockReturn:    "100",
			mockError:     nil,
			validator:     func(float64) error { return nil },
			expectedFloat: 100.0,
			expectError:   false,
		},
		{
			name:          "valid scientific notation",
			mockReturn:    "1.5e2",
			mockError:     nil,
			validator:     func(float64) error { return nil },
			expectedFloat: 150.0,
			expectError:   false,
		},
		{
			name:          "float with failing validator",
			mockReturn:    "50.5",
			mockError:     nil,
			validator:     func(val float64) error { return fmt.Errorf("value %.1f not allowed", val) },
			expectedFloat: 0,
			expectError:   true,
			errorContains: "value 50.5 not allowed",
		},
		{
			name:          "invalid format - not a number",
			mockReturn:    "abc",
			mockError:     nil,
			validator:     func(float64) error { return nil },
			expectedFloat: 0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			validator:     func(float64) error { return nil },
			expectedFloat: 0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			validator:     func(float64) error { return nil },
			expectedFloat: 0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "validation parsing failure - strconv.ParseFloat fails in Validate",
			mockReturn:    "not-a-number",
			mockError:     nil,
			validator:     func(float64) error { return nil },
			expectedFloat: 0,
			expectError:   true,
			errorContains: "strconv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, "Enter float:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "not allowed"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "strconv"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			floatVal, err := prompter.CaptureFloat("Enter float:", tt.validator)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, 0.0, floatVal)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedFloat, floatVal)
			}
		})
	}
}

func TestCapturePositiveIntWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	// Mock comparator for testing
	mockComparator := func(minVal int) comparator.Comparator {
		return comparator.Comparator{
			Label: fmt.Sprintf("min_%d", minVal),
			Type:  comparator.MoreThanEq,
			Value: uint64(minVal),
		}
	}

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		comparators   []comparator.Comparator
		expectedInt   int
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid positive integer without comparators",
			mockReturn:  "42",
			mockError:   nil,
			comparators: nil,
			expectedInt: 42,
			expectError: false,
		},
		{
			name:        "valid zero",
			mockReturn:  "0",
			mockError:   nil,
			comparators: nil,
			expectedInt: 0,
			expectError: false,
		},
		{
			name:        "valid large positive integer",
			mockReturn:  "1000000",
			mockError:   nil,
			comparators: nil,
			expectedInt: 1000000,
			expectError: false,
		},
		{
			name:        "valid integer with passing comparator",
			mockReturn:  "50",
			mockError:   nil,
			comparators: []comparator.Comparator{mockComparator(10)},
			expectedInt: 50,
			expectError: false,
		},
		{
			name:          "invalid - negative integer",
			mockReturn:    "-1",
			mockError:     nil,
			comparators:   nil,
			expectedInt:   0,
			expectError:   true,
			errorContains: "input is less than 0",
		},
		{
			name:          "integer failing comparator validation",
			mockReturn:    "5",
			mockError:     nil,
			comparators:   []comparator.Comparator{mockComparator(10)},
			expectedInt:   0,
			expectError:   true,
			errorContains: "the value must be bigger than or equal to",
		},
		{
			name:          "invalid format - not a number",
			mockReturn:    "abc",
			mockError:     nil,
			comparators:   nil,
			expectedInt:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - float",
			mockReturn:    "42.5",
			mockError:     nil,
			comparators:   nil,
			expectedInt:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			comparators:   nil,
			expectedInt:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			comparators:   nil,
			expectedInt:   0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "validation parsing failure - strconv.Atoi fails in Validate",
			mockReturn:    "not-a-number",
			mockError:     nil,
			comparators:   nil,
			expectedInt:   0,
			expectError:   true,
			errorContains: "strconv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, "Enter positive int:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "input is less than 0") ||
						strings.Contains(tt.errorContains, "the value must be bigger than or equal to"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "strconv"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			intVal, err := prompter.CapturePositiveInt("Enter positive int:", tt.comparators)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, 0, intVal)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedInt, intVal)
			}
		})
	}
}

func TestCaptureUint64CompareWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		comparators   []comparator.Comparator
		expectedVal   uint64
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid uint64 - no comparators",
			mockReturn:  "123",
			mockError:   nil,
			comparators: []comparator.Comparator{},
			expectedVal: 123,
			expectError: false,
		},
		{
			name:        "valid uint64 - decimal format",
			mockReturn:  "456",
			mockError:   nil,
			comparators: []comparator.Comparator{},
			expectedVal: 456,
			expectError: false,
		},
		{
			name:        "valid uint64 - hex format",
			mockReturn:  "0xFF",
			mockError:   nil,
			comparators: []comparator.Comparator{},
			expectedVal: 255,
			expectError: false,
		},
		{
			name:        "valid uint64 - octal format",
			mockReturn:  "0755",
			mockError:   nil,
			comparators: []comparator.Comparator{},
			expectedVal: 493,
			expectError: false,
		},
		{
			name:        "zero value",
			mockReturn:  "0",
			mockError:   nil,
			comparators: []comparator.Comparator{},
			expectedVal: 0,
			expectError: false,
		},
		{
			name:          "invalid format - negative",
			mockReturn:    "-1",
			mockError:     nil,
			comparators:   []comparator.Comparator{},
			expectedVal:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - letters",
			mockReturn:    "abc",
			mockError:     nil,
			comparators:   []comparator.Comparator{},
			expectedVal:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "invalid format - float",
			mockReturn:    "123.45",
			mockError:     nil,
			comparators:   []comparator.Comparator{},
			expectedVal:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			comparators:   []comparator.Comparator{},
			expectedVal:   0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			comparators:   []comparator.Comparator{},
			expectedVal:   0,
			expectError:   true,
			errorContains: "invalid syntax",
		},
		{
			name:          "validation parsing failure - strconv.ParseUint fails in Validate",
			mockReturn:    "not-a-number",
			mockError:     nil,
			comparators:   []comparator.Comparator{},
			expectedVal:   0,
			expectError:   true,
			errorContains: "strconv",
		},
		{
			name:       "comparator validation failure - value too small",
			mockReturn: "5",
			mockError:  nil,
			comparators: []comparator.Comparator{
				{
					Label: "minimum value",
					Type:  comparator.MoreThanEq,
					Value: uint64(10),
				},
			},
			expectedVal:   0,
			expectError:   true,
			errorContains: "the value must be bigger than or equal to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter uint64:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "strconv"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "the value must be bigger than or equal to"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "invalid syntax"):
						return tt.mockReturn, nil
					default:
						err := prompt.Validate(tt.mockReturn)
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			val, err := prompter.CaptureUint64Compare("Enter uint64:", tt.comparators)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, uint64(0), val)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedVal, val)
			}
		})
	}
}

func TestCapturePositiveBigIntWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedVal   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid positive big int",
			mockReturn:  "123456789012345678901234567890",
			mockError:   nil,
			expectedVal: "123456789012345678901234567890",
			expectError: false,
		},
		{
			name:        "zero value",
			mockReturn:  "0",
			mockError:   nil,
			expectedVal: "0",
			expectError: false,
		},
		{
			name:        "small positive number",
			mockReturn:  "42",
			mockError:   nil,
			expectedVal: "42",
			expectError: false,
		},
		{
			name:        "very large number",
			mockReturn:  "999999999999999999999999999999999999999999999999999999999999",
			mockError:   nil,
			expectedVal: "999999999999999999999999999999999999999999999999999999999999",
			expectError: false,
		},
		{
			name:          "negative number",
			mockReturn:    "-123",
			mockError:     nil,
			expectedVal:   "",
			expectError:   true,
			errorContains: "invalid number",
		},
		{
			name:          "invalid format - letters",
			mockReturn:    "abc123",
			mockError:     nil,
			expectedVal:   "",
			expectError:   true,
			errorContains: "invalid number",
		},
		{
			name:          "invalid format - float",
			mockReturn:    "123.45",
			mockError:     nil,
			expectedVal:   "",
			expectError:   true,
			errorContains: "invalid number",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedVal:   "",
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedVal:   "",
			expectError:   true,
			errorContains: "invalid number",
		},
		{
			name:          "SetString fails in function",
			mockReturn:    "invalid-for-setstring",
			mockError:     nil,
			expectedVal:   "",
			expectError:   true,
			errorContains: "SetString: error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter positive big integer:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "invalid number"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case tt.expectError && tt.mockReturn == "":
						// For empty string, validation should fail
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case strings.Contains(tt.errorContains, "SetString: error"):
						// Skip validation to allow the string through, so SetString fails in the actual function
						return tt.mockReturn, nil
					default:
						if tt.mockReturn != "" {
							err := prompt.Validate(tt.mockReturn)
							require.NoError(t, err)
						}
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			val, err := prompter.CapturePositiveBigInt("Enter positive big integer:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Nil(t, val)
			} else {
				require.NoError(t, err)
				require.NotNil(t, val)
				require.Equal(t, tt.expectedVal, val.String())
			}
		})
	}
}

func TestCapturePChainAddressWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		network       string
		expectedAddr  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid custom P-Chain address",
			mockReturn:   "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			mockError:    nil,
			network:      "devnet",
			expectedAddr: "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			expectError:  false,
		},
		{
			name:          "invalid P-Chain address format",
			mockReturn:    "invalid-address",
			mockError:     nil,
			network:       "devnet",
			expectedAddr:  "",
			expectError:   true,
			errorContains: "invalid bech32 string length",
		},
		{
			name:          "non P-Chain address",
			mockReturn:    "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			mockError:     nil,
			network:       "devnet",
			expectedAddr:  "",
			expectError:   true,
			errorContains: "not a PChain address",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			network:       "devnet",
			expectedAddr:  "",
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			network:       "devnet",
			expectedAddr:  "",
			expectError:   true,
			errorContains: "no separator found in address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter P-Chain address:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "invalid bech32 string length") ||
						strings.Contains(tt.errorContains, "not a PChain address") ||
						strings.Contains(tt.errorContains, "no separator found in address"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case tt.expectError && tt.mockReturn == "":
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					default:
						if tt.mockReturn != "" {
							err := prompt.Validate(tt.mockReturn)
							require.NoError(t, err)
						}
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			var network models.Network
			switch tt.network {
			case "devnet":
				network = models.NewDevnetNetwork("", 0)
			case "fuji":
				network = models.NewFujiNetwork()
			case "mainnet":
				network = models.NewMainnetNetwork()
			default:
				network = models.NewLocalNetwork()
			}

			addr, err := prompter.CapturePChainAddress("Enter P-Chain address:", network)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, addr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedAddr, addr)
			}
		})
	}
}

func TestCaptureXChainAddressWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		network       string
		expectedAddr  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid custom X-Chain address",
			mockReturn:   "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			mockError:    nil,
			network:      "devnet",
			expectedAddr: "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			expectError:  false,
		},
		{
			name:          "invalid X-Chain address format",
			mockReturn:    "invalid-address",
			mockError:     nil,
			network:       "devnet",
			expectedAddr:  "",
			expectError:   true,
			errorContains: "invalid bech32 string length",
		},
		{
			name:          "non X-Chain address",
			mockReturn:    "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			mockError:     nil,
			network:       "devnet",
			expectedAddr:  "",
			expectError:   true,
			errorContains: "not a XChain address",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			network:       "devnet",
			expectedAddr:  "",
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			network:       "devnet",
			expectedAddr:  "",
			expectError:   true,
			errorContains: "no separator found in address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter X-Chain address:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "invalid bech32 string length") ||
						strings.Contains(tt.errorContains, "not a XChain address") ||
						strings.Contains(tt.errorContains, "no separator found in address"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case tt.expectError && tt.mockReturn == "":
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					default:
						if tt.mockReturn != "" {
							err := prompt.Validate(tt.mockReturn)
							require.NoError(t, err)
						}
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			var network models.Network
			switch tt.network {
			case "devnet":
				network = models.NewDevnetNetwork("", 0)
			case "fuji":
				network = models.NewFujiNetwork()
			case "mainnet":
				network = models.NewMainnetNetwork()
			default:
				network = models.NewLocalNetwork()
			}

			addr, err := prompter.CaptureXChainAddress("Enter X-Chain address:", network)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, addr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedAddr, addr)
			}
		})
	}
}

func TestCaptureAddressWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedAddr  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid ethereum address with 0x prefix",
			mockReturn:   "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			mockError:    nil,
			expectedAddr: "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			expectError:  false,
		},
		{
			name:         "valid address all lowercase",
			mockReturn:   "0x742d35cc1634c0532925a3b8d400bbcfcc09fbbf",
			mockError:    nil,
			expectedAddr: "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			expectError:  false,
		},
		{
			name:         "valid address without 0x prefix",
			mockReturn:   "742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			mockError:    nil,
			expectedAddr: "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			expectError:  false,
		},
		{
			name:          "invalid address - too short",
			mockReturn:    "0x742d35Cc",
			mockError:     nil,
			expectedAddr:  "",
			expectError:   true,
			errorContains: "invalid address",
		},
		{
			name:          "invalid address - too long",
			mockReturn:    "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF1234",
			mockError:     nil,
			expectedAddr:  "",
			expectError:   true,
			errorContains: "invalid address",
		},
		{
			name:          "invalid address - invalid characters",
			mockReturn:    "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbG",
			mockError:     nil,
			expectedAddr:  "",
			expectError:   true,
			errorContains: "invalid address",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedAddr:  "",
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedAddr:  "",
			expectError:   true,
			errorContains: "invalid address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter Ethereum address:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "invalid address"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case tt.expectError && tt.mockReturn == "":
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					default:
						if tt.mockReturn != "" {
							err := prompt.Validate(tt.mockReturn)
							require.NoError(t, err)
						}
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			addr, err := prompter.CaptureAddress("Enter Ethereum address:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, "0x0000000000000000000000000000000000000000", addr.Hex())
			} else {
				require.NoError(t, err)
				require.Equal(t, strings.ToLower(tt.expectedAddr), strings.ToLower(addr.Hex()))
			}
		})
	}
}

func TestCaptureExistingFilepathWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test_existing_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	err = tmpFile.Close()
	require.NoError(t, err)

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedPath  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid existing file",
			mockReturn:   tmpFile.Name(),
			mockError:    nil,
			expectedPath: tmpFile.Name(),
			expectError:  false,
		},
		{
			name:          "non-existing file",
			mockReturn:    "/path/to/nonexistent/file.txt",
			mockError:     nil,
			expectedPath:  "",
			expectError:   true,
			errorContains: "file doesn't exist",
		},
		{
			name:          "empty string",
			mockReturn:    "",
			mockError:     nil,
			expectedPath:  "",
			expectError:   true,
			errorContains: "file doesn't exist",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedPath:  "",
			expectError:   true,
			errorContains: "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter existing file path:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "file doesn't exist"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					case tt.expectError && tt.mockReturn == "":
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					default:
						if tt.mockReturn != "" {
							err := prompt.Validate(tt.mockReturn)
							require.NoError(t, err)
						}
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			path, err := prompter.CaptureExistingFilepath("Enter existing file path:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, path)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPath, path)
			}
		})
	}
}

func TestCaptureNewFilepathWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	// Create a temporary file that exists
	tmpFile, err := os.CreateTemp("", "test_existing_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	err = tmpFile.Close()
	require.NoError(t, err)

	tests := []struct {
		name          string
		mockReturn    string
		mockError     error
		expectedPath  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid new file path",
			mockReturn:   "/tmp/new_file_that_doesnt_exist.txt",
			mockError:    nil,
			expectedPath: "/tmp/new_file_that_doesnt_exist.txt",
			expectError:  false,
		},
		{
			name:         "new file in temp directory",
			mockReturn:   filepath.Join(os.TempDir(), "new_test_file.txt"),
			mockError:    nil,
			expectedPath: filepath.Join(os.TempDir(), "new_test_file.txt"),
			expectError:  false,
		},
		{
			name:          "existing file",
			mockReturn:    tmpFile.Name(),
			mockError:     nil,
			expectedPath:  "",
			expectError:   true,
			errorContains: "file already exists",
		},
		{
			name:          "prompt error - user cancelled",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedPath:  "",
			expectError:   true,
			errorContains: "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, "Enter new file path:", prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockError == nil {
					switch {
					case strings.Contains(tt.errorContains, "file already exists"):
						err := prompt.Validate(tt.mockReturn)
						require.Error(t, err)
						return "", err
					default:
						if tt.mockReturn != "" {
							err := prompt.Validate(tt.mockReturn)
							require.NoError(t, err)
						}
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			path, err := prompter.CaptureNewFilepath("Enter new file path:")

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, path)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedPath, path)
			}
		})
	}
}

func TestYesNoBaseWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUISelectRunner
	defer func() {
		promptUISelectRunner = originalRunner
	}()

	tests := []struct {
		name           string
		promptStr      string
		orderedOptions []string
		mockIndex      int
		mockDecision   string
		mockError      error
		expectedResult bool
		expectError    bool
		errorContains  string
	}{
		{
			name:           "select Yes (first option)",
			promptStr:      "Do you want to continue?",
			orderedOptions: []string{Yes, No},
			mockIndex:      0,
			mockDecision:   Yes,
			mockError:      nil,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "select No (second option)",
			promptStr:      "Do you want to continue?",
			orderedOptions: []string{Yes, No},
			mockIndex:      1,
			mockDecision:   No,
			mockError:      nil,
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "select No (first option in No/Yes order)",
			promptStr:      "Do you want to continue?",
			orderedOptions: []string{No, Yes},
			mockIndex:      0,
			mockDecision:   No,
			mockError:      nil,
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "select Yes (second option in No/Yes order)",
			promptStr:      "Do you want to continue?",
			orderedOptions: []string{No, Yes},
			mockIndex:      1,
			mockDecision:   Yes,
			mockError:      nil,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "prompt error - user cancelled",
			promptStr:      "Do you want to continue?",
			orderedOptions: []string{Yes, No},
			mockIndex:      0,
			mockDecision:   "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedResult: false,
			expectError:    true,
			errorContains:  "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUISelectRunner = func(prompt promptui.Select) (int, string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, tt.promptStr, prompt.Label)
				require.Equal(t, tt.orderedOptions, prompt.Items)

				return tt.mockIndex, tt.mockDecision, tt.mockError
			}

			result, err := yesNoBase(tt.promptStr, tt.orderedOptions)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.False(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestCaptureYesNoWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUISelectRunner
	defer func() {
		promptUISelectRunner = originalRunner
	}()

	tests := []struct {
		name           string
		promptStr      string
		mockIndex      int
		mockDecision   string
		mockError      error
		expectedResult bool
		expectError    bool
		errorContains  string
	}{
		{
			name:           "select Yes",
			promptStr:      "Do you want to proceed?",
			mockIndex:      0,
			mockDecision:   Yes,
			mockError:      nil,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "select No",
			promptStr:      "Do you want to proceed?",
			mockIndex:      1,
			mockDecision:   No,
			mockError:      nil,
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "prompt error - user cancelled",
			promptStr:      "Do you want to proceed?",
			mockIndex:      0,
			mockDecision:   "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedResult: false,
			expectError:    true,
			errorContains:  "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUISelectRunner = func(prompt promptui.Select) (int, string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, tt.promptStr, prompt.Label)
				require.Equal(t, []string{Yes, No}, prompt.Items)

				return tt.mockIndex, tt.mockDecision, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureYesNo(tt.promptStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.False(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestCaptureNoYesWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUISelectRunner
	defer func() {
		promptUISelectRunner = originalRunner
	}()

	tests := []struct {
		name           string
		promptStr      string
		mockIndex      int
		mockDecision   string
		mockError      error
		expectedResult bool
		expectError    bool
		errorContains  string
	}{
		{
			name:           "select No (first option)",
			promptStr:      "Do you want to proceed?",
			mockIndex:      0,
			mockDecision:   No,
			mockError:      nil,
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "select Yes (second option)",
			promptStr:      "Do you want to proceed?",
			mockIndex:      1,
			mockDecision:   Yes,
			mockError:      nil,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "prompt error - user cancelled",
			promptStr:      "Do you want to proceed?",
			mockIndex:      0,
			mockDecision:   "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedResult: false,
			expectError:    true,
			errorContains:  "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUISelectRunner = func(prompt promptui.Select) (int, string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, tt.promptStr, prompt.Label)
				require.Equal(t, []string{No, Yes}, prompt.Items)

				return tt.mockIndex, tt.mockDecision, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureNoYes(tt.promptStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.False(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestCaptureListWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUISelectRunner
	defer func() {
		promptUISelectRunner = originalRunner
	}()

	tests := []struct {
		name           string
		promptStr      string
		options        []string
		mockIndex      int
		mockDecision   string
		mockError      error
		expectedResult string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "select first option",
			promptStr:      "Choose an option:",
			options:        []string{"Option A", "Option B", "Option C"},
			mockIndex:      0,
			mockDecision:   "Option A",
			mockError:      nil,
			expectedResult: "Option A",
			expectError:    false,
		},
		{
			name:           "select middle option",
			promptStr:      "Choose an option:",
			options:        []string{"Option A", "Option B", "Option C"},
			mockIndex:      1,
			mockDecision:   "Option B",
			mockError:      nil,
			expectedResult: "Option B",
			expectError:    false,
		},
		{
			name:           "select last option",
			promptStr:      "Choose an option:",
			options:        []string{"Option A", "Option B", "Option C"},
			mockIndex:      2,
			mockDecision:   "Option C",
			mockError:      nil,
			expectedResult: "Option C",
			expectError:    false,
		},
		{
			name:           "single option",
			promptStr:      "Only one choice:",
			options:        []string{"Only Option"},
			mockIndex:      0,
			mockDecision:   "Only Option",
			mockError:      nil,
			expectedResult: "Only Option",
			expectError:    false,
		},
		{
			name:           "many options",
			promptStr:      "Choose from many:",
			options:        []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"},
			mockIndex:      7,
			mockDecision:   "H",
			mockError:      nil,
			expectedResult: "H",
			expectError:    false,
		},
		{
			name:           "prompt error - user cancelled",
			promptStr:      "Choose an option:",
			options:        []string{"Option A", "Option B"},
			mockIndex:      0,
			mockDecision:   "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedResult: "",
			expectError:    true,
			errorContains:  "user cancelled",
		},
		{
			name:           "prompt error - selection failed",
			promptStr:      "Choose an option:",
			options:        []string{"Option A", "Option B"},
			mockIndex:      0,
			mockDecision:   "",
			mockError:      fmt.Errorf("selection failed"),
			expectedResult: "",
			expectError:    true,
			errorContains:  "selection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUISelectRunner = func(prompt promptui.Select) (int, string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, tt.promptStr, prompt.Label)
				require.Equal(t, tt.options, prompt.Items)

				return tt.mockIndex, tt.mockDecision, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureList(tt.promptStr, tt.options)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestCaptureListWithSizeWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUISelectRunner
	defer func() {
		promptUISelectRunner = originalRunner
	}()

	tests := []struct {
		name           string
		promptStr      string
		options        []string
		size           int
		mockIndex      int
		mockDecision   string
		mockError      error
		expectedResult string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "select with size 3",
			promptStr:      "Choose an option:",
			options:        []string{"Option A", "Option B", "Option C", "Option D", "Option E"},
			size:           3,
			mockIndex:      1,
			mockDecision:   "Option B",
			mockError:      nil,
			expectedResult: "Option B",
			expectError:    false,
		},
		{
			name:           "select with size 5",
			promptStr:      "Choose from list:",
			options:        []string{"A", "B", "C", "D", "E", "F", "G", "H"},
			size:           5,
			mockIndex:      4,
			mockDecision:   "E",
			mockError:      nil,
			expectedResult: "E",
			expectError:    false,
		},
		{
			name:           "select with size 1",
			promptStr:      "One at a time:",
			options:        []string{"First", "Second", "Third"},
			size:           1,
			mockIndex:      0,
			mockDecision:   "First",
			mockError:      nil,
			expectedResult: "First",
			expectError:    false,
		},
		{
			name:           "select with size larger than options",
			promptStr:      "Big size:",
			options:        []string{"Only", "Two"},
			size:           10,
			mockIndex:      1,
			mockDecision:   "Two",
			mockError:      nil,
			expectedResult: "Two",
			expectError:    false,
		},
		{
			name:           "select with size 0 (should still work)",
			promptStr:      "Zero size:",
			options:        []string{"Option 1", "Option 2"},
			size:           0,
			mockIndex:      0,
			mockDecision:   "Option 1",
			mockError:      nil,
			expectedResult: "Option 1",
			expectError:    false,
		},
		{
			name:           "many options with small size",
			promptStr:      "Scroll through:",
			options:        []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L"},
			size:           4,
			mockIndex:      8,
			mockDecision:   "I",
			mockError:      nil,
			expectedResult: "I",
			expectError:    false,
		},
		{
			name:           "prompt error - user cancelled",
			promptStr:      "Choose with size:",
			options:        []string{"Option A", "Option B"},
			size:           2,
			mockIndex:      0,
			mockDecision:   "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedResult: "",
			expectError:    true,
			errorContains:  "user cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace the global function with mock
			promptUISelectRunner = func(prompt promptui.Select) (int, string, error) {
				// Verify the prompt was set up correctly
				require.Equal(t, tt.promptStr, prompt.Label)
				require.Equal(t, tt.options, prompt.Items)
				require.Equal(t, tt.size, prompt.Size)

				return tt.mockIndex, tt.mockDecision, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureListWithSize(tt.promptStr, tt.options, tt.size)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestCaptureAddressesWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalReadLongString := utilsReadLongString
	defer func() {
		utilsReadLongString = originalReadLongString
	}()

	tests := []struct {
		name              string
		promptStr         string
		mockInputs        []string // Multiple inputs to simulate validation loop
		mockErrors        []error  // Corresponding errors for each input
		expectedAddresses []string // Expected hex addresses
		expectError       bool
		errorContains     string
	}{
		{
			name:              "single valid address",
			promptStr:         "Enter addresses:",
			mockInputs:        []string{"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			mockErrors:        []error{nil},
			expectedAddresses: []string{"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			expectError:       false,
		},
		{
			name:       "multiple valid addresses",
			promptStr:  "Enter addresses:",
			mockInputs: []string{"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF,0x8ba1f109551bD432803012645eac136c108Ba132"},
			mockErrors: []error{nil},
			expectedAddresses: []string{
				"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
				"0x8ba1f109551bD432803012645eac136c108Ba132",
			},
			expectError: false,
		},
		{
			name:       "addresses with spaces (should be trimmed)",
			promptStr:  "Enter addresses:",
			mockInputs: []string{" 0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF , 0x8ba1f109551bD432803012645eac136c108Ba132 "},
			mockErrors: []error{nil},
			expectedAddresses: []string{
				"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
				"0x8ba1f109551bD432803012645eac136c108Ba132",
			},
			expectError: false,
		},
		{
			name:       "validation loop - invalid then valid",
			promptStr:  "Enter addresses:",
			mockInputs: []string{"invalid-address", "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			mockErrors: []error{nil, nil},
			expectedAddresses: []string{
				"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			},
			expectError: false,
		},
		{
			name:       "validation loop - multiple invalid attempts then valid",
			promptStr:  "Enter addresses:",
			mockInputs: []string{"bad1", "bad2,also-bad", "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			mockErrors: []error{nil, nil, nil},
			expectedAddresses: []string{
				"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			},
			expectError: false,
		},
		{
			name:       "mixed valid and invalid addresses in single input (fails validation)",
			promptStr:  "Enter addresses:",
			mockInputs: []string{"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF,invalid", "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			mockErrors: []error{nil, nil},
			expectedAddresses: []string{
				"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			},
			expectError: false,
		},
		{
			name:              "ReadLongString error on first attempt",
			promptStr:         "Enter addresses:",
			mockInputs:        []string{""},
			mockErrors:        []error{fmt.Errorf("input error")},
			expectedAddresses: nil,
			expectError:       true,
			errorContains:     "input error",
		},
		{
			name:              "ReadLongString error after invalid input",
			promptStr:         "Enter addresses:",
			mockInputs:        []string{"invalid", ""}, // First invalid (continues loop), second has error
			mockErrors:        []error{nil, fmt.Errorf("read error")},
			expectedAddresses: nil,
			expectError:       true,
			errorContains:     "read error",
		},
		{
			name:              "empty address in comma-separated list",
			promptStr:         "Enter addresses:",
			mockInputs:        []string{"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF,,0x8ba1f109551bD432803012645eac136c108Ba132", "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			mockErrors:        []error{nil, nil},
			expectedAddresses: []string{"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			expectError:       false,
		},
		{
			name:              "address without 0x prefix (should work)",
			promptStr:         "Enter addresses:",
			mockInputs:        []string{"742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			mockErrors:        []error{nil},
			expectedAddresses: []string{"0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF"},
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0

			// Replace the utils.ReadLongString function with mock
			utilsReadLongString = func(msg string, args ...interface{}) (string, error) {
				// Verify the prompt message format
				expectedMsg := promptui.IconGood + " " + tt.promptStr + " "
				require.Equal(t, expectedMsg, msg)

				// Return mock input based on call count
				if callCount < len(tt.mockInputs) {
					input := tt.mockInputs[callCount]
					err := tt.mockErrors[callCount]
					callCount++
					return input, err
				}

				// If we run out of mock inputs, return an error to prevent infinite loop
				return "", fmt.Errorf("unexpected additional call to ReadLongString")
			}

			prompter := &realPrompter{}
			addresses, err := prompter.CaptureAddresses(tt.promptStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Nil(t, addresses)
			} else {
				require.NoError(t, err)
				require.NotNil(t, addresses)
				require.Len(t, addresses, len(tt.expectedAddresses))

				// Convert addresses to hex strings for comparison
				for i, addr := range addresses {
					require.Equal(t, strings.ToLower(tt.expectedAddresses[i]), strings.ToLower(addr.Hex()))
				}
			}
		})
	}
}

func TestCaptureEmailWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name          string
		promptStr     string
		mockReturn    string
		mockError     error
		expectedEmail string
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid email",
			promptStr:     "Enter email:",
			mockReturn:    "test@example.com",
			mockError:     nil,
			expectedEmail: "test@example.com",
			expectError:   false,
		},
		{
			name:          "valid email with subdomain",
			promptStr:     "Enter email:",
			mockReturn:    "user@mail.example.com",
			mockError:     nil,
			expectedEmail: "user@mail.example.com",
			expectError:   false,
		},
		{
			name:          "valid email with plus sign",
			promptStr:     "Enter email:",
			mockReturn:    "user+test@example.com",
			mockError:     nil,
			expectedEmail: "user+test@example.com",
			expectError:   false,
		},
		{
			name:          "valid email with dots",
			promptStr:     "Enter email:",
			mockReturn:    "first.last@example.com",
			mockError:     nil,
			expectedEmail: "first.last@example.com",
			expectError:   false,
		},
		{
			name:          "invalid email - no @",
			promptStr:     "Enter email:",
			mockReturn:    "testexample.com",
			mockError:     nil,
			expectedEmail: "",
			expectError:   true,
			errorContains: "mail",
		},
		{
			name:          "invalid email - no domain",
			promptStr:     "Enter email:",
			mockReturn:    "test@",
			mockError:     nil,
			expectedEmail: "",
			expectError:   true,
			errorContains: "mail",
		},
		{
			name:          "invalid email - no local part",
			promptStr:     "Enter email:",
			mockReturn:    "@example.com",
			mockError:     nil,
			expectedEmail: "",
			expectError:   true,
			errorContains: "mail",
		},
		{
			name:          "invalid email - double @",
			promptStr:     "Enter email:",
			mockReturn:    "test@@example.com",
			mockError:     nil,
			expectedEmail: "",
			expectError:   true,
			errorContains: "mail",
		},
		{
			name:          "invalid email - spaces",
			promptStr:     "Enter email:",
			mockReturn:    "test @example.com",
			mockError:     nil,
			expectedEmail: "",
			expectError:   true,
			errorContains: "mail",
		},
		{
			name:          "empty string (mail.ParseAddress accepts empty)",
			promptStr:     "Enter email:",
			mockReturn:    "",
			mockError:     nil,
			expectedEmail: "",
			expectError:   false,
		},
		{
			name:          "prompt error - user cancelled",
			promptStr:     "Enter email:",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedEmail: "",
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "prompt error - interrupt",
			promptStr:     "Enter email:",
			mockReturn:    "",
			mockError:     fmt.Errorf("interrupt"),
			expectedEmail: "",
			expectError:   true,
			errorContains: "interrupt",
		},
		{
			name:          "validation error in validateEmail function",
			promptStr:     "Enter email:",
			mockReturn:    "invalid-email-format",
			mockError:     nil,
			expectedEmail: "",
			expectError:   true,
			errorContains: "mail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, tt.promptStr, prompt.Label)
				require.NotNil(t, prompt.Validate)

				if tt.mockReturn != "" && tt.mockError == nil {
					err := prompt.Validate(tt.mockReturn)
					if tt.expectError && !strings.Contains(tt.errorContains, "user cancelled") && !strings.Contains(tt.errorContains, "interrupt") {
						require.Error(t, err)
						return "", err
					} else if !tt.expectError {
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			email, err := prompter.CaptureEmail(tt.promptStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, email)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedEmail, email)
			}
		})
	}
}
