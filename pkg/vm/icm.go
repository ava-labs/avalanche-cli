// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	_ "embed"
	"math/big"
	"strings"

	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
)

const (
	messengerContractAddress = "0x253b2784c75e510dD0fF1da844684a1aC0aa5fcf"
	messengerDeployerAddress = "0x618FEdD9A45a8C456812ecAAE70C671c6249DfaC"
	registryContractAddress  = "0xF86Cb19Ad8405AEFa7d09C778215D2Cb6eBfB228"
)

//go:embed deployed_messenger_bytecode.txt
var deployedMessengerBytecode []byte

//go:embed deployed_registry_bytecode.txt
var deployedRegistryBytecode []byte

func addICMContractToGenesisAllocations(
	allocs core.GenesisAlloc,
) {
	storage := map[common.Hash]common.Hash{
		common.HexToHash("0x0"): common.HexToHash("0x1"),
		common.HexToHash("0x1"): common.HexToHash("0x1"),
	}
	deployedMessengerBytes := common.FromHex(strings.TrimSpace(string(deployedMessengerBytecode)))
	allocs[common.HexToAddress(messengerContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedMessengerBytes,
		Storage: storage,
		Nonce:   1,
	}
	allocs[common.HexToAddress(messengerDeployerAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Nonce:   1,
	}
}

func addICMRegistryContractToGenesisAllocations(
	allocs core.GenesisAlloc,
) {
	storage := map[common.Hash]common.Hash{
		common.HexToHash("0x0"): common.HexToHash("0x1"),
		common.HexToHash("0x1"): common.HexToHash("0x1"),
	}
	deployedRegistryBytes := common.FromHex(strings.TrimSpace(string(deployedRegistryBytecode)))
	allocs[common.HexToAddress(registryContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedRegistryBytes,
		Storage: storage,
		Nonce:   1,
	}
}
