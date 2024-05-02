// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
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
