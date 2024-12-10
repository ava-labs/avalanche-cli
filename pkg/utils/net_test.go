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
		{"127.0.0.1:8080", true},    // valid IP:port
		{"256.0.0.1:8080", false},   // invalid IP address
		{"example.com:8080", false}, // only ip address is allowed
		{"127.0.0.1", false},        // missing port
		{"[::1]:8080", true},        // valid IPv6 address
		{"[::1]", false},            // missing port for IPv6
		{"", false},                 // empty string
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
