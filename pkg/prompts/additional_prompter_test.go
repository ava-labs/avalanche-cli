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

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts/comparator"
	"github.com/manifoldco/promptui"
	"github.com/stretchr/testify/require"
)

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
			utilsReadLongString = func(msg string, _ ...interface{}) (string, error) {
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

func TestCaptureStringAllowEmptyWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name           string
		promptStr      string
		mockReturn     string
		mockError      error
		expectedString string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid non-empty string",
			promptStr:      "Enter string:",
			mockReturn:     "hello world",
			mockError:      nil,
			expectedString: "hello world",
			expectError:    false,
		},
		{
			name:           "empty string (allowed)",
			promptStr:      "Enter string:",
			mockReturn:     "",
			mockError:      nil,
			expectedString: "",
			expectError:    false,
		},
		{
			name:           "string with spaces",
			promptStr:      "Enter description:",
			mockReturn:     "  some text with spaces  ",
			mockError:      nil,
			expectedString: "  some text with spaces  ",
			expectError:    false,
		},
		{
			name:           "string with special characters",
			promptStr:      "Enter string:",
			mockReturn:     "test@example.com & special chars!",
			mockError:      nil,
			expectedString: "test@example.com & special chars!",
			expectError:    false,
		},
		{
			name:           "multiline-like string",
			promptStr:      "Enter string:",
			mockReturn:     "line1\\nline2",
			mockError:      nil,
			expectedString: "line1\\nline2",
			expectError:    false,
		},
		{
			name:           "numeric string",
			promptStr:      "Enter string:",
			mockReturn:     "12345",
			mockError:      nil,
			expectedString: "12345",
			expectError:    false,
		},
		{
			name:           "very long string",
			promptStr:      "Enter string:",
			mockReturn:     "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
			mockError:      nil,
			expectedString: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
			expectError:    false,
		},
		{
			name:           "prompt error - user cancelled",
			promptStr:      "Enter string:",
			mockReturn:     "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedString: "",
			expectError:    true,
			errorContains:  "user cancelled",
		},
		{
			name:           "prompt error - interrupt",
			promptStr:      "Enter string:",
			mockReturn:     "",
			mockError:      fmt.Errorf("interrupt"),
			expectedString: "",
			expectError:    true,
			errorContains:  "interrupt",
		},
		{
			name:           "prompt error - input failed",
			promptStr:      "Enter string:",
			mockReturn:     "",
			mockError:      fmt.Errorf("input failed"),
			expectedString: "",
			expectError:    true,
			errorContains:  "input failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, tt.promptStr, prompt.Label)
				// CaptureStringAllowEmpty should have no validation function
				require.Nil(t, prompt.Validate)

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureStringAllowEmpty(tt.promptStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedString, result)
			}
		})
	}
}

func TestCaptureStringWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name           string
		promptStr      string
		mockReturn     string
		mockError      error
		expectedString string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid non-empty string",
			promptStr:      "Enter string:",
			mockReturn:     "hello world",
			mockError:      nil,
			expectedString: "hello world",
			expectError:    false,
		},
		{
			name:           "valid string with special characters",
			promptStr:      "Enter string:",
			mockReturn:     "test@example.com",
			mockError:      nil,
			expectedString: "test@example.com",
			expectError:    false,
		},
		{
			name:           "valid string with numbers",
			promptStr:      "Enter string:",
			mockReturn:     "test123",
			mockError:      nil,
			expectedString: "test123",
			expectError:    false,
		},
		{
			name:           "empty string - validation fails",
			promptStr:      "Enter string:",
			mockReturn:     "",
			mockError:      nil,
			expectedString: "",
			expectError:    true,
			errorContains:  "string cannot be empty",
		},
		{
			name:           "string with only spaces - valid (validateNonEmpty only checks for empty)",
			promptStr:      "Enter string:",
			mockReturn:     "   ",
			mockError:      nil,
			expectedString: "   ",
			expectError:    false,
		},
		{
			name:           "prompt error - user cancelled",
			promptStr:      "Enter string:",
			mockReturn:     "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedString: "",
			expectError:    true,
			errorContains:  "user cancelled",
		},
		{
			name:           "prompt error - interrupt",
			promptStr:      "Enter string:",
			mockReturn:     "",
			mockError:      fmt.Errorf("interrupt"),
			expectedString: "",
			expectError:    true,
			errorContains:  "interrupt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, tt.promptStr, prompt.Label)
				require.NotNil(t, prompt.Validate)

				// Test validation function if no error expected from prompt
				if tt.mockError == nil {
					err := prompt.Validate(tt.mockReturn)
					if tt.expectError && strings.Contains(tt.errorContains, "string cannot be empty") {
						require.Error(t, err)
						return "", err
					} else if !tt.expectError {
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureString(tt.promptStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedString, result)
			}
		})
	}
}

func TestCaptureValidatedStringWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name           string
		promptStr      string
		validator      func(string) error
		mockReturn     string
		mockError      error
		expectedString string
		expectError    bool
		errorContains  string
	}{
		{
			name:      "valid string passes validator",
			promptStr: "Enter string:",
			validator: func(s string) error {
				if s == "valid" {
					return nil
				}
				return fmt.Errorf("string must be 'valid'")
			},
			mockReturn:     "valid",
			mockError:      nil,
			expectedString: "valid",
			expectError:    false,
		},
		{
			name:      "valid email validator",
			promptStr: "Enter email:",
			validator: func(s string) error {
				if strings.Contains(s, "@") {
					return nil
				}
				return fmt.Errorf("must contain @")
			},
			mockReturn:     "test@example.com",
			mockError:      nil,
			expectedString: "test@example.com",
			expectError:    false,
		},
		{
			name:      "length validator passes",
			promptStr: "Enter string:",
			validator: func(s string) error {
				if len(s) >= 5 {
					return nil
				}
				return fmt.Errorf("string must be at least 5 characters")
			},
			mockReturn:     "testing",
			mockError:      nil,
			expectedString: "testing",
			expectError:    false,
		},
		{
			name:      "string fails validation",
			promptStr: "Enter string:",
			validator: func(s string) error {
				if s == "valid" {
					return nil
				}
				return fmt.Errorf("string must be 'valid'")
			},
			mockReturn:     "invalid",
			mockError:      nil,
			expectedString: "",
			expectError:    true,
			errorContains:  "string must be 'valid'",
		},
		{
			name:      "email validator fails",
			promptStr: "Enter email:",
			validator: func(s string) error {
				if strings.Contains(s, "@") {
					return nil
				}
				return fmt.Errorf("must contain @")
			},
			mockReturn:     "invalid-email",
			mockError:      nil,
			expectedString: "",
			expectError:    true,
			errorContains:  "must contain @",
		},
		{
			name:      "length validator fails",
			promptStr: "Enter string:",
			validator: func(s string) error {
				if len(s) >= 5 {
					return nil
				}
				return fmt.Errorf("string must be at least 5 characters")
			},
			mockReturn:     "hi",
			mockError:      nil,
			expectedString: "",
			expectError:    true,
			errorContains:  "string must be at least 5 characters",
		},
		{
			name:           "prompt error - user cancelled",
			promptStr:      "Enter string:",
			validator:      func(string) error { return nil },
			mockReturn:     "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedString: "",
			expectError:    true,
			errorContains:  "user cancelled",
		},
		{
			name:           "prompt error - interrupt",
			promptStr:      "Enter string:",
			validator:      func(string) error { return nil },
			mockReturn:     "",
			mockError:      fmt.Errorf("interrupt"),
			expectedString: "",
			expectError:    true,
			errorContains:  "interrupt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, tt.promptStr, prompt.Label)
				require.NotNil(t, prompt.Validate)

				// Test validation function if no error expected from prompt
				if tt.mockError == nil {
					err := prompt.Validate(tt.mockReturn)
					if tt.expectError && !strings.Contains(tt.errorContains, "user") && !strings.Contains(tt.errorContains, "interrupt") {
						require.Error(t, err)
						return "", err
					} else if !tt.expectError {
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureValidatedString(tt.promptStr, tt.validator)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedString, result)
			}
		})
	}
}

func TestCaptureGitURLWithMonkeyPatch(t *testing.T) {
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
		expectedURL   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid HTTP URL",
			promptStr:   "Enter Git URL:",
			mockReturn:  "http://github.com/user/repo",
			mockError:   nil,
			expectedURL: "http://github.com/user/repo",
			expectError: false,
		},
		{
			name:        "valid HTTPS URL",
			promptStr:   "Enter Git URL:",
			mockReturn:  "https://github.com/user/repo",
			mockError:   nil,
			expectedURL: "https://github.com/user/repo",
			expectError: false,
		},
		{
			name:        "valid URL with path",
			promptStr:   "Enter Git URL:",
			mockReturn:  "https://github.com/ava-labs/avalanche-cli.git",
			mockError:   nil,
			expectedURL: "https://github.com/ava-labs/avalanche-cli.git",
			expectError: false,
		},
		{
			name:          "invalid URL format - validation fails",
			promptStr:     "Enter Git URL:",
			mockReturn:    "not-a-url",
			mockError:     nil,
			expectedURL:   "",
			expectError:   true,
			errorContains: "invalid",
		},
		{
			name:          "empty string - validation fails",
			promptStr:     "Enter Git URL:",
			mockReturn:    "",
			mockError:     nil,
			expectedURL:   "",
			expectError:   true,
			errorContains: "empty url",
		},
		{
			name:          "malformed URL - validation fails",
			promptStr:     "Enter Git URL:",
			mockReturn:    "http://[::1",
			mockError:     nil,
			expectedURL:   "",
			expectError:   true,
			errorContains: "missing ']' in host",
		},
		{
			name:          "prompt error - user cancelled",
			promptStr:     "Enter Git URL:",
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedURL:   "",
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "prompt error - interrupt",
			promptStr:     "Enter Git URL:",
			mockReturn:    "",
			mockError:     fmt.Errorf("interrupt"),
			expectedURL:   "",
			expectError:   true,
			errorContains: "interrupt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, tt.promptStr, prompt.Label)
				require.NotNil(t, prompt.Validate)

				// Test validation function if no error expected from prompt
				if tt.mockError == nil {
					err := prompt.Validate(tt.mockReturn)
					if tt.expectError && (strings.Contains(tt.errorContains, "empty url") || strings.Contains(tt.errorContains, "missing") || strings.Contains(tt.errorContains, "invalid")) {
						require.Error(t, err)
						return "", err
					} else if !tt.expectError {
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureGitURL(tt.promptStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedURL, result.String())
			}
		})
	}
}

func TestCaptureVersionWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	tests := []struct {
		name            string
		promptStr       string
		mockReturn      string
		mockError       error
		expectedVersion string
		expectError     bool
		errorContains   string
	}{
		{
			name:            "valid semantic version",
			promptStr:       "Enter version:",
			mockReturn:      "v1.0.0",
			mockError:       nil,
			expectedVersion: "v1.0.0",
			expectError:     false,
		},
		{
			name:            "invalid semantic version without v prefix - validation fails",
			promptStr:       "Enter version:",
			mockReturn:      "1.2.3",
			mockError:       nil,
			expectedVersion: "",
			expectError:     true,
			errorContains:   "version must be a legal semantic version",
		},
		{
			name:            "valid pre-release version",
			promptStr:       "Enter version:",
			mockReturn:      "v1.0.0-alpha",
			mockError:       nil,
			expectedVersion: "v1.0.0-alpha",
			expectError:     false,
		},
		{
			name:            "valid build metadata version",
			promptStr:       "Enter version:",
			mockReturn:      "v1.0.0+20210101",
			mockError:       nil,
			expectedVersion: "v1.0.0+20210101",
			expectError:     false,
		},
		{
			name:            "invalid version format - validation fails",
			promptStr:       "Enter version:",
			mockReturn:      "not-a-version",
			mockError:       nil,
			expectedVersion: "",
			expectError:     true,
			errorContains:   "version must be a legal semantic version",
		},
		{
			name:            "incomplete version - validation fails",
			promptStr:       "Enter version:",
			mockReturn:      "1.0",
			mockError:       nil,
			expectedVersion: "",
			expectError:     true,
			errorContains:   "version must be a legal semantic version",
		},
		{
			name:            "empty string - validation fails",
			promptStr:       "Enter version:",
			mockReturn:      "",
			mockError:       nil,
			expectedVersion: "",
			expectError:     true,
			errorContains:   "version must be a legal semantic version",
		},
		{
			name:            "prompt error - user cancelled",
			promptStr:       "Enter version:",
			mockReturn:      "",
			mockError:       fmt.Errorf("user cancelled"),
			expectedVersion: "",
			expectError:     true,
			errorContains:   "user cancelled",
		},
		{
			name:            "prompt error - interrupt",
			promptStr:       "Enter version:",
			mockReturn:      "",
			mockError:       fmt.Errorf("interrupt"),
			expectedVersion: "",
			expectError:     true,
			errorContains:   "interrupt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, tt.promptStr, prompt.Label)
				require.NotNil(t, prompt.Validate)

				// Test validation function if no error expected from prompt
				if tt.mockError == nil {
					err := prompt.Validate(tt.mockReturn)
					if tt.expectError && strings.Contains(tt.errorContains, "version must be a legal semantic version") {
						require.Error(t, err)
						return "", err
					} else if !tt.expectError {
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureVersion(tt.promptStr)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Empty(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedVersion, result)
			}
		})
	}
}

func TestCaptureIndexWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalSelectRunner := promptUISelectRunner
	defer func() {
		promptUISelectRunner = originalSelectRunner
	}()

	tests := []struct {
		name          string
		promptStr     string
		options       []any
		mockIndex     int
		mockValue     string
		mockError     error
		expectedIndex int
		expectError   bool
		errorContains string
	}{
		{
			name:          "select first option",
			promptStr:     "Choose option:",
			options:       []any{"option1", "option2", "option3"},
			mockIndex:     0,
			mockValue:     "option1",
			mockError:     nil,
			expectedIndex: 0,
			expectError:   false,
		},
		{
			name:          "select middle option",
			promptStr:     "Choose option:",
			options:       []any{"option1", "option2", "option3"},
			mockIndex:     1,
			mockValue:     "option2",
			mockError:     nil,
			expectedIndex: 1,
			expectError:   false,
		},
		{
			name:          "select last option",
			promptStr:     "Choose option:",
			options:       []any{"option1", "option2", "option3"},
			mockIndex:     2,
			mockValue:     "option3",
			mockError:     nil,
			expectedIndex: 2,
			expectError:   false,
		},
		{
			name:          "single option",
			promptStr:     "Choose option:",
			options:       []any{"only-option"},
			mockIndex:     0,
			mockValue:     "only-option",
			mockError:     nil,
			expectedIndex: 0,
			expectError:   false,
		},
		{
			name:          "numeric options",
			promptStr:     "Choose number:",
			options:       []any{1, 2, 3, 4, 5},
			mockIndex:     2,
			mockValue:     "3",
			mockError:     nil,
			expectedIndex: 2,
			expectError:   false,
		},
		{
			name:          "select error - user cancelled",
			promptStr:     "Choose option:",
			options:       []any{"option1", "option2"},
			mockIndex:     0,
			mockValue:     "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedIndex: 0,
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "select error - interrupt",
			promptStr:     "Choose option:",
			options:       []any{"option1", "option2"},
			mockIndex:     0,
			mockValue:     "",
			mockError:     fmt.Errorf("interrupt"),
			expectedIndex: 0,
			expectError:   true,
			errorContains: "interrupt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUISelectRunner = func(prompt promptui.Select) (int, string, error) {
				require.Equal(t, tt.promptStr, prompt.Label)
				require.Equal(t, tt.options, prompt.Items)

				return tt.mockIndex, tt.mockValue, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureIndex(tt.promptStr, tt.options)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, 0, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedIndex, result)
			}
		})
	}
}

func TestCaptureFutureDateWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalRunner := promptUIRunner
	defer func() {
		promptUIRunner = originalRunner
	}()

	// Use relative times based on current time to ensure tests work
	now := time.Now().UTC().Truncate(time.Second) // Remove nanoseconds for cleaner comparison
	futureTime := now.Add(24 * time.Hour)
	pastTime := now.Add(-24 * time.Hour)
	customMinDate := now.Add(time.Hour) // Custom min date 1 hour from now

	tests := []struct {
		name          string
		promptStr     string
		minDate       time.Time
		mockReturn    string
		mockError     error
		expectedTime  time.Time
		expectError   bool
		errorContains string
	}{
		{
			name:         "valid future date with zero minDate",
			promptStr:    "Enter date:",
			minDate:      time.Time{},
			mockReturn:   futureTime.Format(constants.TimeParseLayout),
			mockError:    nil,
			expectedTime: futureTime,
			expectError:  false,
		},
		{
			name:         "valid future date with custom minDate",
			promptStr:    "Enter date:",
			minDate:      customMinDate,
			mockReturn:   customMinDate.Add(2 * time.Hour).Format(constants.TimeParseLayout),
			mockError:    nil,
			expectedTime: customMinDate.Add(2 * time.Hour),
			expectError:  false,
		},
		{
			name:         "valid date far in future",
			promptStr:    "Enter date:",
			minDate:      time.Time{},
			mockReturn:   now.Add(365 * 24 * time.Hour).Format(constants.TimeParseLayout),
			mockError:    nil,
			expectedTime: now.Add(365 * 24 * time.Hour),
			expectError:  false,
		},
		{
			name:          "invalid date format - validation fails",
			promptStr:     "Enter date:",
			minDate:       time.Time{},
			mockReturn:    "invalid-date-format",
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "parsing time",
		},
		{
			name:          "date in past - validation fails",
			promptStr:     "Enter date:",
			minDate:       time.Time{},
			mockReturn:    pastTime.Format(constants.TimeParseLayout),
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "the provided date is before",
		},
		{
			name:          "date before custom minDate - validation fails",
			promptStr:     "Enter date:",
			minDate:       customMinDate,
			mockReturn:    customMinDate.Add(-30 * time.Minute).Format(constants.TimeParseLayout),
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "the provided date is before",
		},
		{
			name:          "empty string - validation fails",
			promptStr:     "Enter date:",
			minDate:       time.Time{},
			mockReturn:    "",
			mockError:     nil,
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "parsing time",
		},
		{
			name:          "prompt error - user cancelled",
			promptStr:     "Enter date:",
			minDate:       time.Time{},
			mockReturn:    "",
			mockError:     fmt.Errorf("user cancelled"),
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "user cancelled",
		},
		{
			name:          "prompt error - interrupt",
			promptStr:     "Enter date:",
			minDate:       time.Time{},
			mockReturn:    "",
			mockError:     fmt.Errorf("interrupt"),
			expectedTime:  time.Time{},
			expectError:   true,
			errorContains: "interrupt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUIRunner = func(prompt promptui.Prompt) (string, error) {
				require.Equal(t, tt.promptStr, prompt.Label)
				require.NotNil(t, prompt.Validate)

				// Test validation function if no error expected from prompt
				if tt.mockError == nil {
					err := prompt.Validate(tt.mockReturn)
					if tt.expectError && !strings.Contains(tt.errorContains, "user") && !strings.Contains(tt.errorContains, "interrupt") {
						require.Error(t, err)
						return "", err
					} else if !tt.expectError {
						require.NoError(t, err)
					}
				}

				return tt.mockReturn, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.CaptureFutureDate(tt.promptStr, tt.minDate)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					require.Contains(t, err.Error(), tt.errorContains)
				}
				require.Equal(t, time.Time{}, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedTime, result)
			}
		})
	}
}

func TestChooseKeyOrLedgerWithMonkeyPatch(t *testing.T) {
	// Save original function
	originalSelectRunner := promptUISelectRunner
	defer func() {
		promptUISelectRunner = originalSelectRunner
	}()

	tests := []struct {
		name           string
		goal           string
		mockIndex      int
		mockValue      string
		mockError      error
		expectedResult bool
		expectError    bool
		errorContains  string
	}{
		{
			name:           "choose stored key",
			goal:           "for signing",
			mockIndex:      0,
			mockValue:      "Use stored key",
			mockError:      nil,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "choose ledger",
			goal:           "for signing",
			mockIndex:      1,
			mockValue:      "Use ledger",
			mockError:      nil,
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "choose stored key for different goal",
			goal:           "to validate transactions",
			mockIndex:      0,
			mockValue:      "Use stored key",
			mockError:      nil,
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "choose ledger for different goal",
			goal:           "to validate transactions",
			mockIndex:      1,
			mockValue:      "Use ledger",
			mockError:      nil,
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "select error - user cancelled",
			goal:           "for signing",
			mockIndex:      0,
			mockValue:      "",
			mockError:      fmt.Errorf("user cancelled"),
			expectedResult: false,
			expectError:    true,
			errorContains:  "user cancelled",
		},
		{
			name:           "select error - interrupt",
			goal:           "for signing",
			mockIndex:      0,
			mockValue:      "",
			mockError:      fmt.Errorf("interrupt"),
			expectedResult: false,
			expectError:    true,
			errorContains:  "interrupt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptUISelectRunner = func(prompt promptui.Select) (int, string, error) {
				expectedLabel := fmt.Sprintf("Which key should be used %s?", tt.goal)
				require.Equal(t, expectedLabel, prompt.Label)
				require.Equal(t, []string{"Use stored key", "Use ledger"}, prompt.Items)

				return tt.mockIndex, tt.mockValue, tt.mockError
			}

			prompter := &realPrompter{}
			result, err := prompter.ChooseKeyOrLedger(tt.goal)

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
