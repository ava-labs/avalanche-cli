// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestContains(t *testing.T) {
	require := require.New(t)
	strList := []string{"test", "capture", "list"}

	k1, _ := crypto.GenerateKey()
	k2, _ := crypto.GenerateKey()
	k3, _ := crypto.GenerateKey()

	addr1 := crypto.PubkeyToAddress(k1.PublicKey)
	addr2 := crypto.PubkeyToAddress(k2.PublicKey)
	addr3 := crypto.PubkeyToAddress(k3.PublicKey)

	addrList := []common.Address{
		addr1,
		addr2,
	}

	require.True(contains(strList, "test"))
	require.True(contains(strList, "capture"))
	require.True(contains(strList, "list"))
	require.False(contains(strList, "false"))

	require.True(contains(addrList, addr1))
	require.True(contains(addrList, addr2))
	require.False(contains(addrList, addr3))
}
