// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"reflect"
	"testing"
)

func TestMergeJsonMaps(t *testing.T) {
	tests := []struct {
		name     string
		a        map[string]interface{}
		b        map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "no conflict",
			a:    map[string]interface{}{"key1": "value1"},
			b:    map[string]interface{}{"key2": "value2"},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "with conflict",
			a:    map[string]interface{}{"key1": "value1"},
			b:    map[string]interface{}{"key1": "new_value1", "key2": "value2"},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "empty map a",
			a:    map[string]interface{}{},
			b:    map[string]interface{}{"key1": "value1"},
			expected: map[string]interface{}{
				"key1": "value1",
			},
		},
		{
			name: "empty map b",
			a:    map[string]interface{}{"key1": "value1"},
			b:    map[string]interface{}{},
			expected: map[string]interface{}{
				"key1": "value1",
			},
		},
		{
			name:     "both maps empty",
			a:        map[string]interface{}{},
			b:        map[string]interface{}{},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeJSONMaps(tt.a, tt.b)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("MergeJsonMaps(%v, %v) = %v; expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}
