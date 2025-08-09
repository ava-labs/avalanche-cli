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

// CreateSubnetTxParams contains all parameters needed to create a CreateSubnetTx
type CreateSubnetTxParams struct {
	// ControlKeys are the addresses that can be used to authorize subnet operations
	ControlKeys []string
	// Threshold is the number of control keys needed to authorize a subnet operation
	Threshold int
}

// NewCreateSubnetTx creates uncommitted CreateSubnetTx
// account(keychain) inside the client(wallet) will be used to build the transaction
func NewCreateSubnetTx(
	client wallet.Wallet,
	createSubnetTxParams CreateSubnetTxParams,
) (*multisig.Multisig, error) {
	addrs, err := address.ParseToIDs(createSubnetTxParams.ControlKeys)
	if err != nil {
		return nil, fmt.Errorf("error parsing control keys: %w", err)
	}
	owners := &secp256k1fx.OutputOwners{
		Addrs:     addrs,
		Threshold: uint32(createSubnetTxParams.Threshold),
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
