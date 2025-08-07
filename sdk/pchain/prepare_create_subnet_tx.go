// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package pchain

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/sdk/multisig"
	"github.com/ava-labs/avalanche-cli/sdk/wallet"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// PrepareCreateSubnetTx creates uncommitted CreateSubnetTx
// account(keychain) inside the client(wallet) will be used to build the transaction
func PrepareCreateSubnetTx(
	client wallet.Wallet,
	controlKeys []string,
	threshold int,
) (*multisig.Multisig, error) {
	addrs, err := address.ParseToIDs(controlKeys)
	if err != nil {
		return nil, fmt.Errorf("error parsing control keys: %w", err)
	}
	owners := &secp256k1fx.OutputOwners{
		Addrs:     addrs,
		Threshold: uint32(threshold),
		Locktime:  0,
	}
	unsignedTx, err := client.P().Builder().NewCreateSubnetTx(
		owners,
	)
	if err != nil {
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	return multisig.New(&tx), nil
}
