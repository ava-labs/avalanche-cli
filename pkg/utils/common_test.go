package utils

import (
	"reflect"
	"testing"
)

func TestSplitKeyValueStringToMap(t *testing.T) {
	// Test case 1: Splitting a string with multiple key-value pairs separated by delimiter
	input1 := "key1=value1,key2=value2,key3=value3"
	expected1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	result1 := SplitKeyValueStringToMap(input1, ",")
	if !reflect.DeepEqual(result1, expected1) {
		t.Errorf("Expected %v, but got %v", expected1, result1)
	}

	// Test case 2: Splitting a string with a single key-value pair separated by delimiter
	input2 := "key=value"
	expected2 := map[string]string{
		"key": "value",
	}
	result2 := SplitKeyValueStringToMap(input2, ",")
	if !reflect.DeepEqual(result2, expected2) {
		t.Errorf("Expected %v, but got %v", expected2, result2)
	}

	// Test case 3: Splitting a string with no key-value pairs
	input3 := ""
	expected3 := map[string]string{}
	result3 := SplitKeyValueStringToMap(input3, ",")
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
	result4 := SplitKeyValueStringToMap(input4, ",")
	if !reflect.DeepEqual(result4, expected4) {
		t.Errorf("Expected %v, but got %v", expected4, result4)
	}
}
