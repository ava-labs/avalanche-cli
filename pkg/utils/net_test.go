// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"testing"
)

func TestIsValidIPPort(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"127.0.0.1:8080", true},       // valid IP:port
		{"256.0.0.1:8080", false},      // invalid IP address
		{"127.0.0.1:9650:8080", false}, // only ip address is allowed
		{"127.0.0.1", false},           // missing port
		{"[::1]:8080", true},           // valid IPv6 address
		{"[::1]", false},               // missing port for IPv6
		{"", false},                    // empty string
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := IsValidIPPort(test.input)
			if result != test.expected {
				t.Errorf("Expected IsValidIPPort(%s) to be %v, but got %v", test.input, test.expected, result)
			}
		})
	}
}

func TestSplitRPCURI(t *testing.T) {
	tests := []struct {
		name             string
		requestURI       string
		expectedEndpoint string
		expectedChain    string
		expectError      bool
	}{
		{
			name:             "Valid URI",
			requestURI:       "http://127.0.0.1:9660/ext/bc/nL95ujcHLPFhuQdHYkvS3CSUvDr9EfZduzyJ5Ty6VXXMgyEEF/rpc",
			expectedEndpoint: "http://127.0.0.1:9660",
			expectedChain:    "nL95ujcHLPFhuQdHYkvS3CSUvDr9EfZduzyJ5Ty6VXXMgyEEF",
			expectError:      false,
		},
		{
			name:             "Valid URI with https",
			requestURI:       "https://example.com:8080/ext/bc/testChain/rpc",
			expectedEndpoint: "https://example.com:8080",
			expectedChain:    "testChain",
			expectError:      false,
		},
		{
			name:             "Invalid URI - missing /rpc",
			requestURI:       "http://127.0.0.1:9660/ext/bc/nL95ujcHLPFhuQdHYkvS3CSUvDr9EfZduzyJ5Ty6VXXMgyEEF",
			expectedEndpoint: "",
			expectedChain:    "",
			expectError:      true,
		},
		{
			name:             "Invalid URI - missing /ext/bc/",
			requestURI:       "http://127.0.0.1:9660/some/other/path/rpc",
			expectedEndpoint: "",
			expectedChain:    "",
			expectError:      true,
		},
		{
			name:             "Invalid URI - malformed URL",
			requestURI:       "127.0.0.1:9660/ext/bc/chainId/rpc",
			expectedEndpoint: "",
			expectedChain:    "",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, chain, err := SplitRPCURI(tt.requestURI)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("did not expect an error but got: %v", err)
				}
				if endpoint != tt.expectedEndpoint {
					t.Errorf("expected Endpoint: %s, got: %s", tt.expectedEndpoint, endpoint)
				}
				if chain != tt.expectedChain {
					t.Errorf("expected Chain: %s, got: %s", tt.expectedChain, chain)
				}
			}
		})
	}
}
