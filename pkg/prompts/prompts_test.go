// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	assert := assert.New(t)
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

	assert.True(contains(strList, "test"))
	assert.True(contains(strList, "capture"))
	assert.True(contains(strList, "list"))
	assert.False(contains(strList, "false"))

	assert.True(contains(addrList, addr1))
	assert.True(contains(addrList, addr2))
	assert.False(contains(addrList, addr3))
}
