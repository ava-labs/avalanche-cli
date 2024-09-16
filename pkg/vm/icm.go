// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
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

func setSimpleStorageValue(
	storage map[common.Hash]common.Hash,
	slot string,
	value string,
) {
	storage[common.HexToHash(slot)] = common.HexToHash(value)
}

func trimHexa(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

func hexFill32(s string) string {
	return fmt.Sprintf("%064s", trimHexa(s))
}

func setMappingStorageValue(
	storage map[common.Hash]common.Hash,
	slot string,
	key string,
	value string,
) error {
	slot = hexFill32(slot)
	key = hexFill32(key)
	storageKey := key + slot
	storageKeyBytes, err := hex.DecodeString(storageKey)
	if err != nil {
		return err
	}
	storage[crypto.Keccak256Hash(storageKeyBytes)] = common.HexToHash(value)
	return nil
}

func addICMContractToGenesisAllocations(
	allocs core.GenesisAlloc,
) {
	storage := map[common.Hash]common.Hash{}
	setSimpleStorageValue(storage, "0", "1")
	setSimpleStorageValue(storage, "1", "1")
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
) error {
	storage := map[common.Hash]common.Hash{}
	setSimpleStorageValue(storage, "0", "1")
	if err := setMappingStorageValue(storage, "1", "1", messengerContractAddress); err != nil {
		return err
	}
	if err := setMappingStorageValue(storage, "2", messengerContractAddress, "2"); err != nil {
		return err
	}
	deployedRegistryBytes := common.FromHex(strings.TrimSpace(string(deployedRegistryBytecode)))
	allocs[common.HexToAddress(registryContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedRegistryBytes,
		Storage: storage,
		Nonce:   1,
	}
	return nil
}
