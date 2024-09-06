// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"reflect"
	"testing"
)

func TestExtractPlaceholderValue(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		text     string
		expected string
		wantErr  bool
	}{
		{
			name:     "Extract Version",
			pattern:  `avaplatform/avalanchego:(\S+)`,
			text:     "avaplatform/avalanchego:v1.14.4",
			expected: "v1.14.4",
			wantErr:  false,
		},
		{
			name:     "Extract File Path",
			pattern:  `config\.file=(\S+)`,
			text:     "promtail -config.file=/etc/promtail/promtail.yaml",
			expected: "/etc/promtail/promtail.yaml",
			wantErr:  false,
		},
		{
			name:     "No Match",
			pattern:  `nonexistent=(\S+)`,
			text:     "image: avaplatform/avalanchego:v1.14.4",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractPlaceholderValue(tt.pattern, tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractPlaceholderValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ExtractPlaceholderValue() = %v, want %v", got, tt.expected)
			}
		})
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
