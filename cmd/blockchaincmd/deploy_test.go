// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/stretchr/testify/require"
)

const (
	testAvagoVersion1      = "v1.9.2"
	testAvagoVersion2      = "v1.9.1"
	testLatestAvagoVersion = "latest"
)

var testAvagoCompat = []byte("{\"19\": [\"v1.9.2\"],\"18\": [\"v1.9.1\"],\"17\": [\"v1.9.0\",\"v1.8.0\"]}")

func TestMutuallyExclusive(t *testing.T) {
	require := require.New(t)
	type test struct {
		flagA       bool
		flagB       bool
		flagC       bool
		expectError bool
	}

	tests := []test{
		{
			flagA:       false,
			flagB:       false,
			flagC:       false,
			expectError: false,
		},
		{
			flagA:       true,
			flagB:       false,
			flagC:       false,
			expectError: false,
		},
		{
			flagA:       false,
			flagB:       true,
			flagC:       false,
			expectError: false,
		},
		{
			flagA:       false,
			flagB:       false,
			flagC:       true,
			expectError: false,
		},
		{
			flagA:       true,
			flagB:       false,
			flagC:       true,
			expectError: true,
		},
		{
			flagA:       false,
			flagB:       true,
			flagC:       true,
			expectError: true,
		},
		{
			flagA:       true,
			flagB:       true,
			flagC:       false,
			expectError: true,
		},
		{
			flagA:       true,
			flagB:       true,
			flagC:       true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		isEx := flags.EnsureMutuallyExclusive([]bool{tt.flagA, tt.flagB, tt.flagC})
		if tt.expectError {
			require.False(isEx)
		} else {
			require.True(isEx)
		}
	}
}
