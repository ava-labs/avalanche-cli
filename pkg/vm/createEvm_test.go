// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/testutils"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func Test_ensureAdminsFunded(t *testing.T) {
	addrs, err := testutils.GenerateEthAddrs(5)
	require.NoError(t, err)

	type test struct {
		name       string
		alloc      core.GenesisAlloc
		admins     []common.Address
		shouldFail bool
	}
	tests := []test{
		{
			name: "One address funded",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[0]: {},
				addrs[1]: {
					Balance: big.NewInt(42),
				},
				addrs[2]: {},
			},
			admins:     []common.Address{addrs[1]},
			shouldFail: false,
		},
		{
			name: "Two addresses funded",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[2]: {},
				addrs[3]: {
					Balance: big.NewInt(42),
				},
				addrs[4]: {
					Balance: big.NewInt(42),
				},
			},
			admins:     []common.Address{addrs[3], addrs[4]},
			shouldFail: false,
		},
		{
			name: "Two addresses in Genesis but no funds",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[0]: {
					Balance: big.NewInt(0),
				},
				addrs[1]: {},
				addrs[2]: {},
			},
			admins:     []common.Address{addrs[0], addrs[2]},
			shouldFail: true,
		},
		{
			name: "No address funded",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[0]: {},
				addrs[1]: {},
				addrs[2]: {},
			},
			admins:     []common.Address{addrs[3], addrs[4]},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			err := ensureAdminsHaveBalance(tt.admins, tt.alloc)
			if tt.shouldFail {
				require.Error(err)
			} else {
				require.NoError(err)
			}
		})
	}
}

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
			require := require.New(t)

			// Check how many selected
			shortenedList, err := removePrecompile(tt.precompileList, tt.toRemove)
			require.Equal(tt.expectedResult, shortenedList)
			require.Equal(tt.expectedErr, err)
		})
	}
}
