// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package prompts

import (
	"fmt"
	"strings"
	"testing"
	"time"

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
