// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"math/big"
	"reflect"
	"testing"
)

func TestAddSingleQuotes(t *testing.T) {
	input := []string{"", "b", "orange banana", "'apple'", "'a", "b'"}
	expected := []string{"''", "'b'", "'orange banana'", "'apple'", "'a'", "'b'"}
	output := AddSingleQuotes(input)

	if !reflect.DeepEqual(output, expected) {
		t.Errorf("AddSingleQuotes(%v) = %v, expected %v", input, output, expected)
	}
}

// TestSpitStringWithQuotes test case
func TestSpitStringWithQuotes(t *testing.T) {
	input1 := " arg1 arg2 'hello world' "
	expected1 := []string{"arg1", "arg2", "'hello world'"}
	result1 := SplitStringWithQuotes(input1, ' ')
	if !reflect.DeepEqual(result1, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, result1)
	}
}

func TestFormatAmount(t *testing.T) {
	testCases := []struct {
		name     string
		amount   uint64
		decimals uint8
		expected string
	}{
		{
			name:     "greater than 1",
			amount:   54321,
			decimals: 3,
			expected: "54.321",
		},
		{
			name:     "less than 1",
			amount:   1,
			decimals: 10,
			expected: "0.0000000001",
		},
		{
			name:     "18 decimals",
			amount:   9988776655443322110,
			decimals: 18,
			expected: "9.988776655443322110",
		},
		{
			name:     "9 decimals, all zeros",
			amount:   5000000000,
			decimals: 9,
			expected: "5.000000000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatAmount(new(big.Int).SetUint64(tc.amount), tc.decimals)
			if result != tc.expected {
				t.Errorf("Expected %s, but got %s", tc.expected, result)
			}
		})
	}
}
