// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
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
		allowList  AllowList
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
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[1]},
			},
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
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[3], addrs[4]},
			},
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
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[0], addrs[2]},
			},
			shouldFail: true,
		},
		{
			name: "No address funded",
			alloc: map[common.Address]core.GenesisAccount{
				addrs[0]: {},
				addrs[1]: {},
				addrs[2]: {},
			},
			allowList: AllowList{
				AdminAddresses: []common.Address{addrs[3], addrs[4]},
			},
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			b := someAllowedHasBalance(tt.allowList, tt.alloc)
			if tt.shouldFail {
				require.Equal(b, false)
			} else {
				require.Equal(b, true)
			}
		})
	}
}
