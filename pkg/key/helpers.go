// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// Package key implements key manager and helper functions.
package key

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	ledger "github.com/ava-labs/avalanche-ledger-go"
	"github.com/ava-labs/avalanchego/utils/crypto/keychain"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/logging"
)

const numLedgerAddressesToDerive = 1

func GetKeychain(
	useLedger bool,
	keyPath string,
	network models.Network,
) (keychain.Keychain, error) {
	// get keychain accesor
	var kc keychain.Keychain
	if useLedger {
		ledgerDevice, err := ledger.New()
		if err != nil {
			return kc, err
		}
		// ask for addresses here to print user msg for ledger interaction
		ux.Logger.PrintToUser("*** Please provide extended public key on the ledger device ***")
		addresses, err := ledgerDevice.Addresses(1)
		if err != nil {
			return kc, err
		}
		addr := addresses[0]
		networkID, err := network.NetworkID()
		if err != nil {
			return kc, err
		}
		addrStr, err := address.Format("P", GetHRP(networkID), addr[:])
		if err != nil {
			return kc, err
		}
		ux.Logger.PrintToUser(logging.Yellow.Wrap(fmt.Sprintf("Ledger address: %s", addrStr)))
		return keychain.NewLedgerKeychain(ledgerDevice, numLedgerAddressesToDerive)
	}
	networkID, err := network.NetworkID()
	if err != nil {
		return kc, err
	}
	sf, err := LoadSoft(networkID, keyPath)
	if err != nil {
		return kc, err
	}
	return sf.KeyChain(), nil
}
