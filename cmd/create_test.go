// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_moreThanOneVMSelected(t *testing.T) {
	type test struct {
		name           string
		useSubnetVM    bool
		useCustomVM    bool
		expectedResult bool
	}
	tests := []test{
		{
			name:           "One Selected",
			useSubnetVM:    true,
			useCustomVM:    false,
			expectedResult: false,
		},
		{
			name:           "One Selected Reverse",
			useSubnetVM:    true,
			useCustomVM:    false,
			expectedResult: false,
		},
		{
			name:           "None Selected",
			useSubnetVM:    false,
			useCustomVM:    false,
			expectedResult: false,
		},
		{
			name:           "Multiple Selected",
			useSubnetVM:    true,
			useCustomVM:    true,
			expectedResult: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			// Set vars
			useSubnetEvm = tt.useSubnetVM
			useCustom = tt.useCustomVM

			// Check how many selected
			result := moreThanOneVMSelected()
			assert.Equal(tt.expectedResult, result)
		})
	}
}
