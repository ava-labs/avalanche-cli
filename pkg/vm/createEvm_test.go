// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_removePrecompile(t *testing.T) {
	allowList := "allow list"
	minter := "minter"

	type test struct {
		name           string
		precompileList []string
		toRemove       string
		expectedResult []string
		expectedErr    error
	}
	tests := []test{
		{
			name:           "Success",
			precompileList: []string{allowList, minter},
			toRemove:       allowList,
			expectedResult: []string{minter},
			expectedErr:    nil,
		},
		{
			name:           "Success reverse",
			precompileList: []string{allowList, minter},
			toRemove:       minter,
			expectedResult: []string{allowList},
			expectedErr:    nil,
		},
		{
			name:           "Failure",
			precompileList: []string{minter},
			toRemove:       allowList,
			expectedResult: []string{minter},
			expectedErr:    errors.New("string not in array"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)

			// Check how many selected
			shortenedList, err := removePrecompile(tt.precompileList, tt.toRemove)
			assert.Equal(tt.expectedResult, shortenedList)
			assert.Equal(tt.expectedErr, err)
		})
	}
}
