// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keychain

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/sdk/key"
	"github.com/ava-labs/avalanche-cli/sdk/ledger"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"golang.org/x/exp/maps"
)

type Keychain struct {
	keychain.Keychain
	Ledger *Ledger
	Keys   []key.Key
}

// LedgerParams is an input to NewKeyChain if a new keychain is to be created using Ledger
//
// To view Ledger addresses and their balances, you can use Avalanche CLI and use the command
// avalanche key list --ledger [0,1,2,3,4]
// The example command above will list the first five addresses in your Ledger
//
// To transfer funds between addresses in Ledger, refer to https://docs.avax.network/tooling/cli-transfer-funds/how-to-transfer-funds
type LedgerParams struct {
	// LedgerAddresses specify which addresses in Ledger should be in the Keychain
	// NewKeyChain will then look for the indexes of the specified addresses and add the indexes
	// into LedgerIndices in Ledger
	LedgerAddresses []string

	// RequiredFunds is the minimum total AVAX that the selected addresses from Ledger should contain.
	// NewKeychain will then look through all indexes of all addresses in the Ledger until
	// sufficient AVAX balance is reached.
	// For example if Ledger's index 0 and index 1 each contains 0.1 AVAX and RequiredFunds is
	// 0.2 AVAX, LedgerIndices will have value of [0,1]
	RequiredFunds uint64
}

// Ledger is part of the output of NewKeyChain if a new keychain is to be created using Ledger
type Ledger struct {
	// LedgerDevice is the main interface of interacting with the Ledger Device
	LedgerDevice *ledger.LedgerDevice

	// LedgerIndices contain indexes of the addresses selected from Ledger
	LedgerIndices []uint32
}

func PrivateKeyToAvalancheAccount(
	privateKey string,
) (*Keychain, error) {
	k, err := key.LoadSoftFromBytes([]byte(privateKey))
	if err != nil {
		return nil, err
	}
	kc := Keychain{
		Keychain: k.KeyChain(),
		Keys:     []key.Key{k},
	}
	return &kc, nil
}

// NewKeychain generates a new key pair from either a stored key path or Ledger.
// For stored keys, NewKeychain will generate a new key pair in the provided keyPath if no .pk
// file currently exists in the provided path.
func NewKeychain(
	network network.Network,
	keyPath string,
	ledgerInfo *LedgerParams,
) (*Keychain, error) {
	if ledgerInfo != nil {
		if keyPath != "" {
			return nil, fmt.Errorf("keychain can only created either from key path or ledger, not both")
		}
		dev, err := ledger.New()
		if err != nil {
			return nil, err
		}
		kc := Keychain{
			Ledger: &Ledger{
				LedgerDevice: dev,
			},
		}
		if ledgerInfo.RequiredFunds > 0 {
			if err := kc.AddLedgerFunds(network, ledgerInfo.RequiredFunds); err != nil {
				return nil, err
			}
		}
		if len(ledgerInfo.LedgerAddresses) > 0 {
			if err := kc.AddLedgerAddresses(ledgerInfo.LedgerAddresses); err != nil {
				return nil, err
			}
		}
		if len(kc.Ledger.LedgerIndices) == 0 {
			return nil, fmt.Errorf("keychain currently does not contain any addresses from ledger")
		}
		return &kc, nil
	}
	sf, err := key.LoadSoftOrCreate(keyPath)
	if err != nil {
		return nil, err
	}
	kc := Keychain{
		Keychain: sf.KeyChain(),
	}
	return &kc, nil
}

func (kc *Keychain) LedgerEnabled() bool {
	return kc.Ledger.LedgerDevice != nil
}

func (kc *Keychain) AddLedgerIndices(indices []uint32) error {
	if kc.LedgerEnabled() {
		kc.Ledger.LedgerIndices = utils.Unique(append(kc.Ledger.LedgerIndices, indices...))
		utils.Uint32Sort(kc.Ledger.LedgerIndices)
		newKc, err := keychain.NewLedgerKeychainFromIndices(kc.Ledger.LedgerDevice, kc.Ledger.LedgerIndices)
		if err != nil {
			return err
		}
		kc.Keychain = newKc
		return nil
	}
	return fmt.Errorf("keychain is not ledger enabled")
}

func (kc *Keychain) AddLedgerAddresses(addresses []string) error {
	if kc.LedgerEnabled() {
		indices, err := kc.Ledger.LedgerDevice.FindAddresses(addresses, 0)
		if err != nil {
			return err
		}
		return kc.AddLedgerIndices(maps.Values(indices))
	}
	return fmt.Errorf("keychain is not ledger enabled")
}

func (kc *Keychain) AddLedgerFunds(
	network network.Network,
	amount uint64,
) error {
	if kc.LedgerEnabled() {
		indices, err := kc.Ledger.LedgerDevice.FindFunds(network, amount, 0)
		if err != nil {
			return err
		}
		return kc.AddLedgerIndices(indices)
	}
	return fmt.Errorf("keychain is not ledger enabled")
}

func (kc *Keychain) P(
	network *network.Network,
) ([]string, error) {
	addrs := kc.Addresses().List()
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses in keychain")
	}
	hrp := network.HRP()
	addrsStr := []string{}
	for _, addr := range addrs {
		addrStr, err := address.Format("P", hrp, addr[:])
		if err != nil {
			return nil, err
		}
		addrsStr = append(addrsStr, addrStr)
	}
	return addrsStr, nil
}

func (kc *Keychain) X(
	network *network.Network,
) ([]string, error) {
	addrs := kc.Addresses().List()
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses in keychain")
	}
	hrp := network.HRP()
	addrsStr := []string{}
	for _, addr := range addrs {
		addrStr, err := address.Format("X", hrp, addr[:])
		if err != nil {
			return nil, err
		}
		addrsStr = append(addrsStr, addrStr)
	}
	return addrsStr, nil
}

func (kc *Keychain) C() ([]string, error) {
	if len(kc.Keys) == 0 {
		return nil, fmt.Errorf("no addresses in keychain")
	}
	addrsStr := []string{}
	for _, k := range kc.Keys {
		addrsStr = append(addrsStr, k.C())
	}
	return addrsStr, nil
}
