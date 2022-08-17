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

	anyStr := make([]any, len(strList))
	anyAddr := make([]any, len(addrList))

	for i, s := range strList {
		anyStr[i] = s
	}

	for i, a := range addrList {
		anyAddr[i] = a
	}

	assert.True(contains(anyStr, "test"))
	assert.True(contains(anyStr, "capture"))
	assert.True(contains(anyStr, "list"))
	assert.False(contains(anyStr, "false"))

	assert.True(contains(anyAddr, addr1))
	assert.True(contains(anyAddr, addr2))
	assert.False(contains(anyAddr, addr3))
}
