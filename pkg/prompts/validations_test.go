// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package prompts

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid email",
			input:   "test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			input:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "invalid email - no @",
			input:   "testexample.com",
			wantErr: true,
		},
		{
			name:    "invalid email - no domain",
			input:   "test@",
			wantErr: true,
		},
		{
			name:    "invalid email - no local part",
			input:   "@example.com",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmail(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatePositiveBigInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid positive integer",
			input:   "123",
			wantErr: false,
		},
		{
			name:    "zero is valid",
			input:   "0",
			wantErr: false,
		},
		{
			name:    "large positive number",
			input:   "999999999999999999999999999999",
			wantErr: false,
		},
		{
			name:    "negative number",
			input:   "-1",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "float number",
			input:   "123.45",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePositiveBigInt(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMainnetStakingDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid duration within range",
			input:   "720h", // 30 days
			wantErr: false,
		},
		{
			name:    "minimum duration",
			input:   genesis.MainnetParams.MinStakeDuration.String(),
			wantErr: false,
		},
		{
			name:    "maximum duration",
			input:   genesis.MainnetParams.MaxStakeDuration.String(),
			wantErr: false,
		},
		{
			name:    "duration too short",
			input:   "1h",
			wantErr: true,
		},
		{
			name:    "duration too long",
			input:   "9600h", // 400 days
			wantErr: true,
		},
		{
			name:    "invalid duration format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMainnetStakingDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMainnetL1StakingDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid duration",
			input:   "720h", // 30 days
			wantErr: false,
		},
		{
			name:    "minimum L1 duration - 24h",
			input:   "24h",
			wantErr: false,
		},
		{
			name:    "maximum duration",
			input:   genesis.MainnetParams.MaxStakeDuration.String(),
			wantErr: false,
		},
		{
			name:    "duration too short - less than 24h",
			input:   "12h",
			wantErr: true,
		},
		{
			name:    "duration too long",
			input:   "9600h", // 400 days
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMainnetL1StakingDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateFujiStakingDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid duration within range",
			input:   "720h", // 30 days
			wantErr: false,
		},
		{
			name:    "minimum duration",
			input:   genesis.FujiParams.MinStakeDuration.String(),
			wantErr: false,
		},
		{
			name:    "maximum duration",
			input:   genesis.FujiParams.MaxStakeDuration.String(),
			wantErr: false,
		},
		{
			name:    "duration too short",
			input:   "1h",
			wantErr: true,
		},
		{
			name:    "duration too long",
			input:   "9600h", // 400 days
			wantErr: true,
		},
		{
			name:    "invalid duration format",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFujiStakingDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid duration - hours",
			input:   "24h",
			wantErr: false,
		},
		{
			name:    "valid duration - minutes",
			input:   "30m",
			wantErr: false,
		},
		{
			name:    "valid duration - seconds",
			input:   "45s",
			wantErr: false,
		},
		{
			name:    "valid duration - days",
			input:   "168h", // 7 days
			wantErr: false,
		},
		{
			name:    "valid duration - complex",
			input:   "1h30m45s",
			wantErr: false,
		},
		{
			name:    "invalid duration format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "just number without unit",
			input:   "123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTime(t *testing.T) {
	// Create test times with sufficient buffer for processing delays
	now := time.Now().UTC()
	// Use a much larger buffer to account for test execution time
	futureTime := now.Add(constants.StakingStartLeadTime + time.Hour)
	pastTime := now.Add(-time.Hour)
	closeTime := now.Add(constants.StakingStartLeadTime - time.Minute)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid time well beyond lead time boundary",
			input:   futureTime.Format(constants.TimeParseLayout),
			wantErr: false,
		},
		{
			name:    "time too close to now",
			input:   closeTime.Format(constants.TimeParseLayout),
			wantErr: true,
		},
		{
			name:    "past time",
			input:   pastTime.Format(constants.TimeParseLayout),
			wantErr: true,
		},
		{
			name:    "invalid time format",
			input:   "invalid-time",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "wrong format",
			input:   "2023-12-25 15:30:00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTime(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateNodeID(t *testing.T) {
	// Generate a valid node ID for testing
	validNodeID := ids.GenerateTestNodeID()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid node ID",
			input:   validNodeID.String(),
			wantErr: false,
		},
		{
			name:    "invalid node ID - too short",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "invalid node ID - wrong format",
			input:   "NodeID-InvalidFormat123",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			input:   "NodeID-@#$%^&*()",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNodeID(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid ethereum address",
			input:   "0x742d35Cc1634C0532925a3b8D400bbCFcc09FbbF",
			wantErr: false,
		},
		{
			name:    "valid address all lowercase",
			input:   "0x742d35cc1634c0532925a3b8d400bbcfcc09fbbf",
			wantErr: false, // Go-ethereum accepts all lowercase
		},
		{
			name:    "invalid address - no 0x prefix",
			input:   "742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			wantErr: false, // libevm IsHexAddress accepts this
		},
		{
			name:    "invalid address - too short",
			input:   "0x742d35Cc",
			wantErr: true,
		},
		{
			name:    "invalid address - too long",
			input:   "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF1234",
			wantErr: true,
		},
		{
			name:    "invalid address - invalid characters",
			input:   "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbG",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateAddresses(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "single valid address",
			input:   "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF",
			wantErr: false,
		},
		{
			name:    "multiple valid addresses",
			input:   "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF,0x8ba1f109551bD432803012645Hac136c",
			wantErr: true, // second address is invalid
		},
		{
			name:    "multiple valid addresses with spaces",
			input:   "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF, 0x8ba1f109551bd432803012645eac136c108ba132",
			wantErr: false,
		},
		{
			name:    "empty address in list",
			input:   "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF,,0x8ba1f109551bD432803012645Hac136c108Ba132",
			wantErr: true,
		},
		{
			name:    "invalid address in list",
			input:   "0x742d35Cc1634C0532925a3b8D400bbcFcc09FbbF,invalid,0x8ba1f109551bD432803012645Hac136c108Ba132",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAddresses(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateExistingFilepath(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "test_file_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	err = tmpFile.Close()
	require.NoError(t, err)

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "test_dir_*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "existing file",
			input:   tmpFile.Name(),
			wantErr: false,
		},
		{
			name:    "non-existing file",
			input:   "/path/to/nonexistent/file.txt",
			wantErr: true,
		},
		{
			name:    "directory instead of file",
			input:   tmpDir,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExistingFilepath(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateValidatorBalanceFunc(t *testing.T) {
	availableBalance := 1000.0
	minBalance := 10.0
	validator := validateValidatorBalanceFunc(availableBalance, minBalance)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid balance within range",
			input:   "50.0",
			wantErr: false,
		},
		{
			name:    "minimum valid balance",
			input:   "10.0",
			wantErr: false,
		},
		{
			name:    "maximum available balance",
			input:   "1000.0",
			wantErr: false,
		},
		{
			name:    "zero balance",
			input:   "0",
			wantErr: true,
		},
		{
			name:    "balance below minimum",
			input:   "5.0",
			wantErr: true,
		},
		{
			name:    "balance above available",
			input:   "1500.0",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateBiggerThanZero(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid positive number",
			input:   "123",
			wantErr: false,
		},
		{
			name:    "valid decimal number",
			input:   "1",
			wantErr: false,
		},
		{
			name:    "hex number",
			input:   "0xFF",
			wantErr: false,
		},
		{
			name:    "octal number",
			input:   "0755",
			wantErr: false,
		},
		{
			name:    "zero value",
			input:   "0",
			wantErr: true,
		},
		{
			name:    "negative number",
			input:   "-1",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "float number",
			input:   "12.34",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBiggerThanZero(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateURLFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid HTTP URL",
			input:   "http://example.com",
			wantErr: false,
		},
		{
			name:    "valid HTTPS URL",
			input:   "https://example.com",
			wantErr: false,
		},
		{
			name:    "valid URL with path",
			input:   "https://example.com/path/to/resource",
			wantErr: false,
		},
		{
			name:    "valid URL with query params",
			input:   "https://example.com/path?param=value",
			wantErr: false,
		},
		{
			name:    "valid URL with port",
			input:   "https://example.com:8080",
			wantErr: false,
		},
		{
			name:    "invalid URL - no scheme",
			input:   "example.com",
			wantErr: true,
		},
		{
			name:    "invalid URL - malformed",
			input:   "http://[::1",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid URL - spaces",
			input:   "https://exa mple.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURLFormat(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatePChainAddress(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedHRP string
		wantErr     bool
	}{
		{
			name:        "valid P-Chain address - ewoq test address",
			input:       "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			expectedHRP: "custom",
			wantErr:     false,
		},
		{
			name:        "valid address format but not P-Chain - X-Chain ewoq",
			input:       "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			expectedHRP: "",
			wantErr:     true, // Parse succeeds but chainID != "P"
		},
		{
			name:        "invalid Fuji P-Chain address - bad checksum",
			input:       "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			expectedHRP: "",
			wantErr:     true, // This will fail checksum validation
		},
		{
			name:        "invalid Mainnet P-Chain address - bad checksum",
			input:       "P-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			expectedHRP: "",
			wantErr:     true, // This will fail checksum validation
		},
		{
			name:        "invalid Local P-Chain address - bad checksum",
			input:       "P-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			expectedHRP: "",
			wantErr:     true, // This will fail checksum validation
		},
		{
			name:        "invalid - not P-Chain (X-Chain prefix)",
			input:       "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			expectedHRP: "",
			wantErr:     true,
		},
		{
			name:        "invalid - not P-Chain (C-Chain prefix)",
			input:       "C-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			expectedHRP: "",
			wantErr:     true,
		},
		{
			name:        "invalid - unknown chain prefix",
			input:       "Z-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			expectedHRP: "",
			wantErr:     true,
		},
		{
			name:        "invalid - no chain prefix",
			input:       "fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			expectedHRP: "",
			wantErr:     true,
		},
		{
			name:        "invalid - malformed address",
			input:       "P-invalid-address",
			expectedHRP: "",
			wantErr:     true,
		},
		{
			name:        "empty string",
			input:       "",
			expectedHRP: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hrp, err := validatePChainAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedHRP, hrp)
			}
		})
	}
}

func TestValidatePChainFujiAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid P-Chain address with Fuji HRP",
			input:   "P-fuji18jma8ppw3nhx5r4ap8clazz0dps7rv5u6wmu4t",
			wantErr: false, // Parse succeeds and HRP == "fuji"
		},
		{
			name:    "valid P-Chain address but wrong HRP - custom",
			input:   "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			wantErr: true, // Parse succeeds but HRP != "fuji"
		},
		{
			name:    "invalid Fuji P-Chain address - bad checksum",
			input:   "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid - Mainnet address",
			input:   "P-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			wantErr: true,
		},
		{
			name:    "invalid - Local address",
			input:   "P-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true,
		},
		{
			name:    "invalid - not P-Chain",
			input:   "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePChainFujiAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatePChainMainAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid P-Chain address with Mainnet HRP",
			input:   "P-avax18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
			wantErr: false, // Parse succeeds and HRP == "avax"
		},
		{
			name:    "valid P-Chain address but wrong HRP - custom",
			input:   "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			wantErr: true, // Parse succeeds but HRP != "avax"
		},
		{
			name:    "invalid Mainnet P-Chain address - bad checksum",
			input:   "P-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid - Fuji address",
			input:   "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true,
		},
		{
			name:    "invalid - Local address",
			input:   "P-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true,
		},
		{
			name:    "invalid - not P-Chain",
			input:   "X-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePChainMainAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatePChainLocalAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid P-Chain address with custom HRP",
			input:   "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			wantErr: false,
		},
		{
			name:    "invalid P-Chain address - unsupported HRP",
			input:   "P-avax18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
			wantErr: true, // HRP is neither local nor fallback
		},
		{
			name:    "invalid Local P-Chain address - bad checksum",
			input:   "P-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid Custom P-Chain address - bad checksum",
			input:   "P-custom1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcwfmrp",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid - Fuji address",
			input:   "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true,
		},
		{
			name:    "invalid - Mainnet address",
			input:   "P-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			wantErr: true,
		},
		{
			name:    "invalid - not P-Chain",
			input:   "X-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePChainLocalAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetPChainValidationFunc(t *testing.T) {
	tests := []struct {
		name        string
		network     models.Network
		validAddr   string
		invalidAddr string
	}{
		{
			name:        "Fuji network",
			network:     models.NewFujiNetwork(),
			validAddr:   "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			invalidAddr: "P-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
		},
		{
			name:        "Mainnet network",
			network:     models.NewMainnetNetwork(),
			validAddr:   "P-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			invalidAddr: "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
		},
		{
			name:        "Local network",
			network:     models.NewLocalNetwork(),
			validAddr:   "P-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			invalidAddr: "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
		},
		{
			name:        "Devnet network",
			network:     models.NewDevnetNetwork("", 0),
			validAddr:   "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			invalidAddr: "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := getPChainValidationFunc(tt.network)

			// Test "valid" address - most fail due to bad checksum, except Devnet
			err := validator(tt.validAddr)
			if tt.network.Kind == models.Devnet {
				require.NoError(t, err) // Devnet address with custom HRP should be valid
			} else {
				require.Error(t, err) // Other test addresses have bad checksums
			}

			// Test invalid address
			err = validator(tt.invalidAddr)
			require.Error(t, err)
		})
	}

	// Test unsupported network
	t.Run("unsupported network", func(t *testing.T) {
		unsupportedNetwork := models.Network{Kind: 999} // Use an invalid numeric value
		validator := getPChainValidationFunc(unsupportedNetwork)
		err := validator("P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported network")
	})
}

func TestValidateXChainAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid X-Chain address",
			input:   "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			wantErr: false,
		},
		{
			name:    "valid address format but not X-Chain",
			input:   "P-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			wantErr: true, // chainID != "X"
		},
		{
			name:    "invalid Fuji X-Chain address - bad checksum",
			input:   "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid Mainnet X-Chain address - bad checksum",
			input:   "X-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid Local X-Chain address - bad checksum",
			input:   "X-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid - not X-Chain",
			input:   "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true,
		},
		{
			name:    "invalid - malformed address",
			input:   "X-invalid-address",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateXChainAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateXChainFujiAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid X-Chain address with Fuji HRP",
			input:   "X-fuji18jma8ppw3nhx5r4ap8clazz0dps7rv5u6wmu4t",
			wantErr: false,
		},
		{
			name:    "valid X-Chain address but wrong HRP - custom",
			input:   "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			wantErr: true, // Parse succeeds but HRP != "fuji"
		},
		{
			name:    "invalid Fuji X-Chain address - bad checksum",
			input:   "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid - Mainnet address",
			input:   "X-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			wantErr: true,
		},
		{
			name:    "invalid - Local address",
			input:   "X-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true,
		},
		{
			name:    "invalid - not X-Chain",
			input:   "P-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateXChainFujiAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateXChainMainAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid X-Chain address with Mainnet HRP",
			input:   "X-avax18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
			wantErr: false,
		},
		{
			name:    "valid X-Chain address but wrong HRP - custom",
			input:   "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			wantErr: true, // Parse succeeds but HRP != "avax"
		},
		{
			name:    "invalid Mainnet X-Chain address - bad checksum",
			input:   "X-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid - Fuji address",
			input:   "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			wantErr: true,
		},
		{
			name:    "invalid - Local address",
			input:   "X-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true,
		},
		{
			name:    "invalid - not X-Chain",
			input:   "P-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateXChainMainAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateXChainLocalAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid X-Chain address with custom HRP",
			input:   "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			wantErr: false,
		},
		{
			name:    "invalid X-Chain address with local HRP - bad checksum",
			input:   "X-local18jma8ppw3nhx5r4ap8clazz0dps7rv5uwdpekrw",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid X-Chain address - unsupported HRP",
			input:   "X-fuji18jma8ppw3nhx5r4ap8clazz0dps7rv5u6wmu4t",
			wantErr: true, // HRP is neither local nor custom
		},
		{
			name:    "invalid X-Chain address - Mainnet HRP",
			input:   "X-avax18jma8ppw3nhx5r4ap8clazz0dps7rv5ukulre5",
			wantErr: true, // HRP is neither local nor custom
		},
		{
			name:    "invalid Local X-Chain address - bad checksum",
			input:   "X-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid Custom X-Chain address - bad checksum",
			input:   "X-custom1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcwfmrp",
			wantErr: true, // This will fail checksum validation
		},
		{
			name:    "invalid - not X-Chain",
			input:   "P-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateXChainLocalAddress(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetXChainValidationFunc(t *testing.T) {
	tests := []struct {
		name        string
		network     models.Network
		validAddr   string
		invalidAddr string
	}{
		{
			name:        "Fuji network",
			network:     models.NewFujiNetwork(),
			validAddr:   "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
			invalidAddr: "X-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
		},
		{
			name:        "Mainnet network",
			network:     models.NewMainnetNetwork(),
			validAddr:   "X-avax1x459sj0ssm4tdrn372f7fhqx7p4pkj9hh8a74w",
			invalidAddr: "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
		},
		{
			name:        "Local network",
			network:     models.NewLocalNetwork(),
			validAddr:   "X-local1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhcz8r9x",
			invalidAddr: "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
		},
		{
			name:        "Devnet network",
			network:     models.NewDevnetNetwork("", 0),
			validAddr:   "X-custom18jma8ppw3nhx5r4ap8clazz0dps7rv5u9xde7p",
			invalidAddr: "X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := getXChainValidationFunc(tt.network)

			// Test "valid" address - most fail due to bad checksum, except Devnet
			err := validator(tt.validAddr)
			if tt.network.Kind == models.Devnet {
				require.NoError(t, err) // Devnet address with custom HRP should be valid
			} else {
				require.Error(t, err) // Other test addresses have bad checksums
			}

			// Test invalid address
			err = validator(tt.invalidAddr)
			require.Error(t, err)
		})
	}

	// Test unsupported network
	t.Run("unsupported network", func(t *testing.T) {
		unsupportedNetwork := models.Network{Kind: 999} // Use an invalid numeric value
		validator := getXChainValidationFunc(unsupportedNetwork)
		err := validator("X-fuji1x459sj0ssm4tdrn372f7fhqx7p4pkj9hhqhmp5")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported network")
	})
}

func TestValidateID(t *testing.T) {
	// Generate a valid ID for testing
	validID := ids.GenerateTestID()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid ID",
			input:   validID.String(),
			wantErr: false,
		},
		{
			name:    "invalid ID - too short",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "invalid ID - wrong format",
			input:   "invalidID123",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid characters",
			input:   "2Z4UuXuKg@#$%^&*()",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateID(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateNewFilepath(t *testing.T) {
	// Create a temporary file that exists
	tmpFile, err := os.CreateTemp("", "existing_file_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	err = tmpFile.Close()
	require.NoError(t, err)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "new file path (doesn't exist)",
			input:   "/tmp/new_file_that_doesnt_exist.txt",
			wantErr: false,
		},
		{
			name:    "existing file",
			input:   tmpFile.Name(),
			wantErr: true,
		},
		{
			name:    "new file in temp directory",
			input:   filepath.Join(os.TempDir(), "new_test_file.txt"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNewFilepath(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateNonEmpty(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "non-empty string",
			input:   "test",
			wantErr: false,
		},
		{
			name:    "string with spaces",
			input:   "   ",
			wantErr: false,
		},
		{
			name:    "single character",
			input:   "a",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNonEmpty(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateHexa(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid hex string",
			input:   "0x123abc",
			wantErr: false,
		},
		{
			name:    "valid hex string - uppercase",
			input:   "0x123ABC",
			wantErr: false,
		},
		{
			name:    "valid hex string - mixed case",
			input:   "0x123aBc",
			wantErr: false,
		},
		{
			name:    "valid hex string - long",
			input:   "0x123456789abcdef0123456789abcdef012345678",
			wantErr: false,
		},
		{
			name:    "invalid - no 0x prefix",
			input:   "123abc",
			wantErr: true,
		},
		{
			name:    "invalid - wrong prefix",
			input:   "1x123abc",
			wantErr: true,
		},
		{
			name:    "invalid - only prefix",
			input:   "0x",
			wantErr: true,
		},
		{
			name:    "invalid - non-hex characters",
			input:   "0x123xyz",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid - spaces",
			input:   "0x123 abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHexa(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRequestURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid URL - GitHub",
			url:     "https://github.com/ava-labs/avalanche-cli",
			wantErr: false,
		},
		{
			name:    "valid URL - Google",
			url:     "https://www.google.com",
			wantErr: false,
		},
		{
			name:    "invalid URL - non-existent domain",
			url:     "https://thisdomaindoesnotexist12345.com",
			wantErr: true,
		},
		{
			name:    "invalid URL - 404 page",
			url:     "https://github.com/ava-labs/avalanche-cli/blob/main/nonexistent-file.txt",
			wantErr: true,
		},
		{
			name:    "invalid URL - malformed",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "invalid URL - missing protocol",
			url:     "github.com",
			wantErr: true,
		},
		{
			name:    "invalid URL - causes NewRequest to fail",
			url:     "http://[::1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := RequestURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.Equal(t, http.StatusOK, resp.StatusCode)
				_ = resp.Body.Close()
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid URL - GitHub",
			url:     "https://github.com/ava-labs/avalanche-cli",
			wantErr: false,
		},
		{
			name:    "valid URL - Google",
			url:     "https://www.google.com",
			wantErr: false,
		},
		{
			name:    "invalid URL format",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "invalid URL - non-existent domain",
			url:     "https://thisdomaindoesnotexist12345.com",
			wantErr: true,
		},
		{
			name:    "invalid URL - 404 page",
			url:     "https://github.com/ava-labs/avalanche-cli/blob/main/nonexistent-file.txt",
			wantErr: true,
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateRepoBranch(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		branch  string
		wantErr bool
	}{
		{
			name:    "valid repo and branch - avalanche-cli main",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "main",
			wantErr: false,
		},
		{
			name:    "valid repo but non-existent branch",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "nonexistent-branch-12345",
			wantErr: true,
		},
		{
			name:    "non-existent repo",
			repo:    "https://github.com/nonexistent-org/nonexistent-repo",
			branch:  "main",
			wantErr: true,
		},
		{
			name:    "invalid repo URL",
			repo:    "not-a-repo-url",
			branch:  "main",
			wantErr: true,
		},
		{
			name:    "empty repo",
			repo:    "",
			branch:  "main",
			wantErr: true,
		},
		{
			name:    "empty branch",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRepoBranch(tt.repo, tt.branch)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateRepoFile(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		branch  string
		file    string
		wantErr bool
	}{
		{
			name:    "valid repo, branch, and file",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "main",
			file:    "README.md",
			wantErr: false,
		},
		{
			name:    "valid repo and branch but non-existent file",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "main",
			file:    "nonexistent-file.txt",
			wantErr: true,
		},
		{
			name:    "valid repo but non-existent branch",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "nonexistent-branch",
			file:    "README.md",
			wantErr: true,
		},
		{
			name:    "non-existent repo",
			repo:    "https://github.com/nonexistent-org/nonexistent-repo",
			branch:  "main",
			file:    "README.md",
			wantErr: true,
		},
		{
			name:    "invalid repo URL",
			repo:    "not-a-repo-url",
			branch:  "main",
			file:    "README.md",
			wantErr: true,
		},
		{
			name:    "empty repo",
			repo:    "",
			branch:  "main",
			file:    "README.md",
			wantErr: true,
		},
		{
			name:    "empty branch",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "",
			file:    "README.md",
			wantErr: true,
		},
		{
			name:    "empty file - GitHub handles gracefully",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "main",
			file:    "",
			wantErr: false, // GitHub redirects empty file to branch view
		},
		{
			name:    "file in subdirectory",
			repo:    "https://github.com/ava-labs/avalanche-cli",
			branch:  "main",
			file:    "cmd/root.go",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRepoFile(tt.repo, tt.branch, tt.file)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateWeightFunc(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid weight within range",
			input:   "50",
			wantErr: false,
		},
		{
			name:    "minimum valid weight",
			input:   "1",
			wantErr: false,
		},
		{
			name:    "large valid weight",
			input:   "100",
			wantErr: false,
		},
		{
			name:    "very large weight",
			input:   "1000",
			wantErr: false,
		},
		{
			name:    "zero weight - below minimum",
			input:   "0",
			wantErr: true,
		},
		{
			name:    "negative weight - invalid format",
			input:   "-1",
			wantErr: true,
		},
		{
			name:    "invalid format - letters",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid format - float",
			input:   "50.5",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format - mixed",
			input:   "50abc",
			wantErr: true,
		},
	}

	// Test without extra validation
	t.Run("without extra validation", func(t *testing.T) {
		validator := validateWeightFunc(nil)
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.input)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	// Test with extra validation that always passes
	t.Run("with extra validation that passes", func(t *testing.T) {
		extraValidation := func(uint64) error {
			return nil // Always pass
		}
		validator := validateWeightFunc(extraValidation)

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.input)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	// Test with extra validation that fails for values > 100
	t.Run("with extra validation max 100", func(t *testing.T) {
		extraValidation := func(val uint64) error {
			if val > 100 {
				return fmt.Errorf("weight must not exceed 100")
			}
			return nil
		}
		validator := validateWeightFunc(extraValidation)

		// Test cases specific to this extra validation
		extraTests := []struct {
			name    string
			input   string
			wantErr bool
		}{
			{
				name:    "weight within extra limit",
				input:   "50",
				wantErr: false,
			},
			{
				name:    "weight at extra limit",
				input:   "100",
				wantErr: false,
			},
			{
				name:    "weight above extra limit",
				input:   "150",
				wantErr: true,
			},
			{
				name:    "large weight above extra limit",
				input:   "1000",
				wantErr: true,
			},
		}

		for _, tt := range extraTests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.input)
				if tt.wantErr {
					require.Error(t, err)
					require.Contains(t, err.Error(), "must not exceed 100")
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	// Test with extra validation that fails for even numbers
	t.Run("with extra validation odd numbers only", func(t *testing.T) {
		extraValidation := func(val uint64) error {
			if val%2 == 0 {
				return fmt.Errorf("weight must be an odd number")
			}
			return nil
		}
		validator := validateWeightFunc(extraValidation)

		// Test cases specific to this extra validation
		extraTests := []struct {
			name    string
			input   string
			wantErr bool
		}{
			{
				name:    "odd number",
				input:   "1",
				wantErr: false,
			},
			{
				name:    "another odd number",
				input:   "99",
				wantErr: false,
			},
			{
				name:    "even number",
				input:   "2",
				wantErr: true,
			},
			{
				name:    "another even number",
				input:   "100",
				wantErr: true,
			},
		}

		for _, tt := range extraTests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator(tt.input)
				if tt.wantErr {
					require.Error(t, err)
					require.Contains(t, err.Error(), "must be an odd number")
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestValidatePositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		input   int
		wantErr bool
	}{
		{
			name:    "positive integer",
			input:   1,
			wantErr: false,
		},
		{
			name:    "large positive integer",
			input:   1000000,
			wantErr: false,
		},
		{
			name:    "zero",
			input:   0,
			wantErr: true,
		},
		{
			name:    "negative integer",
			input:   -1,
			wantErr: true,
		},
		{
			name:    "large negative integer",
			input:   -1000000,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositiveInt(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
