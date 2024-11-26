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

func TestSplitRPCtURI(t *testing.T) {
	tests := []struct {
		name          string
		requestURI    string
		expectedURI   string
		expectedChain string
		expectError   bool
	}{
		{
			name:          "Valid URI without trailing slash",
			requestURI:    "http://127.0.0.1:9650/ext/bc/mychain",
			expectedURI:   "http://127.0.0.1:9650",
			expectedChain: "mychain",
			expectError:   false,
		},
		{
			name:          "Valid URI with trailing slash",
			requestURI:    "http://127.0.0.1:9650/ext/bc/mychain/",
			expectedURI:   "http://127.0.0.1:9650",
			expectedChain: "mychain",
			expectError:   false,
		},
		{
			name:          "Invalid URI - missing /ext/bc/",
			requestURI:    "http://127.0.0.1:9650/mychain",
			expectedURI:   "",
			expectedChain: "",
			expectError:   true,
		},
		{
			name:          "Valid URI with no chain",
			requestURI:    "http://127.0.0.1:9650/ext/bc/",
			expectedURI:   "http://127.0.0.1:9650",
			expectedChain: "",
			expectError:   false,
		},
		{
			name:          "Valid URI with complex chain",
			requestURI:    "http://127.0.0.1:9650/ext/bc/mychain/extra",
			expectedURI:   "http://127.0.0.1:9650",
			expectedChain: "mychain/extra",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, chain, err := SplitRPCURI(tt.requestURI)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("did not expect an error but got: %v", err)
				}
				if uri != tt.expectedURI {
					t.Errorf("expected URI: %s, got: %s", tt.expectedURI, uri)
				}
				if chain != tt.expectedChain {
					t.Errorf("expected Chain: %s, got: %s", tt.expectedChain, chain)
				}
			}
		})
	}
}
