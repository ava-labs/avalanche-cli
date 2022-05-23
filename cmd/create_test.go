// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_moreThanOneVmSelected(t *testing.T) {
	type test struct {
		name           string
		useSubnetVm    bool
		useCustomVm    bool
		expectedResult bool
	}
	tests := []test{
		{
			name:           "One Selected",
			useSubnetVm:    true,
			useCustomVm:    false,
			expectedResult: false,
		},
		{
			name:           "One Selected Reverse",
			useSubnetVm:    true,
			useCustomVm:    false,
			expectedResult: false,
		},
		{
			name:           "None Selected",
			useSubnetVm:    false,
			useCustomVm:    false,
			expectedResult: false,
		},
		{
			name:           "Multiple Selected",
			useSubnetVm:    true,
			useCustomVm:    true,
			expectedResult: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			// Set vars
			useSubnetEvm = tt.useSubnetVm
			useCustom = tt.useCustomVm

			// Check how many selected
			result := moreThanOneVmSelected()
			assert.Equal(tt.expectedResult, result)
		})
	}
}
