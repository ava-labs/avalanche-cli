// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"reflect"
	"testing"
)

// TestSpitStringWithQuotes test case
func TestSpitStringWithQuotes(t *testing.T) {
	input1 := " arg1 arg2 'hello world' "
	expected1 := []string{"arg1", "arg2", "'hello world'"}
	result1 := SplitStringWithQuotes(input1, ' ')
	if !reflect.DeepEqual(result1, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, result1)
	}
}

func TestSplitKeyValueStringToMap(t *testing.T) {
	// Test case 1: Splitting a string with multiple key-value pairs separated by delimiter
	input1 := "key1=value1,key2=value2,key3=value3"
	expected1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	result1, _ := SplitKeyValueStringToMap(input1, ",")
	if !reflect.DeepEqual(result1, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, result1)
	}

	// Test case 2: Splitting a string with a single key-value pair separated by delimiter
	input2 := "key=value"
	expected2 := map[string]string{
		"key": "value",
	}
	result2, _ := SplitKeyValueStringToMap(input2, ",")
	if !reflect.DeepEqual(result2, expected2) {
		t.Errorf("Expected %v, but got %v", expected2, result2)
	}

	// Test case 3: Splitting a string with no key-value pairs
	input3 := ""
	expected3 := map[string]string{}
	result3, _ := SplitKeyValueStringToMap(input3, ",")
	if !reflect.DeepEqual(result3, expected3) {
		t.Errorf("Expected %v, but got %v", expected3, result3)
	}

	// Test case 4: Splitting a string with  partial key-value pairs
	input4 := "key0,key1=value1,key2=value2,key3=value3"
	expected4 := map[string]string{
		"key0": "key0",
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	result4, _ := SplitKeyValueStringToMap(input4, ",")
	if !reflect.DeepEqual(result4, expected4) {
		t.Errorf("Expected %v, but got %v", expected4, result4)
	}

	// Test case 5: real life scenario
	input5 := "aws_node_i-009713a2ebe873b86 ansible_host=127.0.0.1 ansible_user=ubuntu ansible_ssh_private_key_file=/Users/user/.ssh/kp.pem ansible_ssh_common_args='-o IdentitiesOnly=yes -o StrictHostKeyChecking=no'"
	expected5 := map[string]string{
		"aws_node_i-009713a2ebe873b86": "aws_node_i-009713a2ebe873b86",
		"ansible_host":                 "127.0.0.1",
		"ansible_user":                 "ubuntu",
		"ansible_ssh_private_key_file": "/Users/user/.ssh/kp.pem",
		"ansible_ssh_common_args":      "-o IdentitiesOnly=yes -o StrictHostKeyChecking=no",
	}
	result5, _ := SplitKeyValueStringToMap(input5, " ")
	if !reflect.DeepEqual(result5, expected5) {
		t.Errorf("Expected %v, but got %v", expected5, result5)
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{[]string{"apple", "orange", "apple", "banana", "orange"}, []string{"apple", "orange", "banana"}},
		{[]string{"dog", "cat", "dog", "bird", "cat"}, []string{"dog", "cat", "bird"}},
		{[]string{"one", "two", "three", "four", "five"}, []string{"one", "two", "three", "four", "five"}},
		// Add more test cases as needed
	}

	for _, test := range tests {
		result := Unique(test.input)
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("Unique(%v) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestSplitSliceAt(t *testing.T) {
	// Test case 1: Split at index 2
	intSlice := []int{1, 2, 3, 4, 5}
	firstPart, secondPart := SplitSliceAt(intSlice, 2)
	expectedFirstPart := []int{1, 2}
	expectedSecondPart := []int{3, 4, 5}
	if !reflect.DeepEqual(firstPart, expectedFirstPart) {
		t.Errorf("Expected first part %v, but got %v", expectedFirstPart, firstPart)
	}
	if !reflect.DeepEqual(secondPart, expectedSecondPart) {
		t.Errorf("Expected second part %v, but got %v", expectedSecondPart, secondPart)
	}
	// Test case 2: Split at index 0
	firstPart, secondPart = SplitSliceAt(intSlice, 0)
	if firstPart != nil {
		t.Errorf("Expected first part nil, but got %v", firstPart)
	}
	if !reflect.DeepEqual(secondPart, intSlice) {
		t.Errorf("Expected second part %v, but got %v", intSlice, secondPart)
	}

	// Test case 3: Split at index out of bounds
	firstPart, secondPart = SplitSliceAt(intSlice, 10)
	if !reflect.DeepEqual(firstPart, intSlice) {
		t.Errorf("Expected first part %v, but got %v", intSlice, firstPart)
	}
	if secondPart != nil {
		t.Errorf("Expected second part nil, but got %v", secondPart)
	}
}

// TestGetRepoFromCommitURL tests GetRepoFromCommitURL
func TestGetRepoFromCommitURL(t *testing.T) {
	expected1 := "https://github.com/sukantoraymond/subnet-evm"
	expected2 := "subnet-evm"
	gitRepo, dirName := GetRepoFromCommitURL("https://github.com/sukantoraymond/subnet-evm/commit/29979c9c38f15a8e2af1db3102a0b70e03c91ab2")
	if !reflect.DeepEqual(gitRepo, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, gitRepo)
	}
	if !reflect.DeepEqual(dirName, expected2) {
		t.Errorf("Expected %v, but got %v", expected2, dirName)
	}
	expected1 = "https://github.com/ava-labs/hypersdk"
	expected2 = "hypersdk"
	gitRepo, dirName = GetRepoFromCommitURL("https://github.com/ava-labs/hypersdk/pull/772/commits/b88acfb370f5aeb83a000aece2d72f28154410a5")
	if !reflect.DeepEqual(gitRepo, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, gitRepo)
	}
	if !reflect.DeepEqual(dirName, expected2) {
		t.Errorf("Expected %v, but got %v", expected2, dirName)
	}
}

// TestGetGitCommit tests GetGitCommit
func TestGetGitCommit(t *testing.T) {
	expected1 := "29979c9c38f15a8e2af1db3102a0b70e03c91ab2"
	commitID := GetGitCommit("https://github.com/sukantoraymond/subnet-evm/commit/29979c9c38f15a8e2af1db3102a0b70e03c91ab2")
	if !reflect.DeepEqual(commitID, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, commitID)
	}
	expected1 = "b88acfb370f5aeb83a000aece2d72f28154410a5"
	commitID = GetGitCommit("https://github.com/ava-labs/hypersdk/pull/772/commits/b88acfb370f5aeb83a000aece2d72f28154410a5")
	if !reflect.DeepEqual(commitID, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, commitID)
	}
}

func TestAddSingleQuotes(t *testing.T) {
	input := []string{"", "b", "orange banana", "'apple'", "'a", "b'"}
	expected := []string{"''", "'b'", "'orange banana'", "'apple'", "'a'", "'b'"}
	output := AddSingleQuotes(input)

	if !reflect.DeepEqual(output, expected) {
		t.Errorf("AddSingleQuotes(%v) = %v, expected %v", input, output, expected)
	}
}
