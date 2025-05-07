// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/stretchr/testify/require"
)

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
