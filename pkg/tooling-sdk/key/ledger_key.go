// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package key

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

var _ Key = &LedgerKey{}

type LedgerKey struct {
	index uint32
}

// ledger device should be connected
func NewLedger(index uint32) LedgerKey {
	return LedgerKey{
		index: index,
	}
}

// LoadLedger loads the ledger key info from disk and creates the corresponding LedgerKey.
func LoadLedger(_ string) (*LedgerKey, error) {
	return nil, fmt.Errorf("not implemented")
}

// LoadLedgerFromBytes loads the ledger key info from bytes and creates the corresponding LedgerKey.
func LoadLedgerFromBytes(_ []byte) (*SoftKey, error) {
	return nil, fmt.Errorf("not implemented")
}

func (*LedgerKey) C() string {
	return ""
}

// Returns the KeyChain
func (*LedgerKey) KeyChain() *secp256k1fx.Keychain {
	return nil
}

// Saves the key info to disk
func (*LedgerKey) Save(_ string) error {
	return fmt.Errorf("not implemented")
}

func (*LedgerKey) P(_ string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (*LedgerKey) X(_ string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (*LedgerKey) Spends(_ []*avax.UTXO, _ ...OpOption) (
	totalBalanceToSpend uint64,
	inputs []*avax.TransferableInput,
	signers [][]ids.ShortID,
) {
	return 0, nil, nil
}

func (*LedgerKey) Addresses() []ids.ShortID {
	return nil
}

func (*LedgerKey) Sign(_ *txs.Tx, _ [][]ids.ShortID) error {
	return fmt.Errorf("not implemented")
}

func (*LedgerKey) Match(_ *secp256k1fx.OutputOwners, _ uint64) ([]uint32, []ids.ShortID, bool) {
	return nil, nil, false
}
