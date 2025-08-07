// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package pchain

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/sdk/multisig"
	"github.com/ava-labs/avalanche-cli/sdk/wallet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

// PrepareCreateChainTx creates uncommitted CreateChainTx
// keychain in wallet will be used to build the transaction
func PrepareCreateChainTx(
	client wallet.Wallet,
	subnetIDStr string,
	vmIDStr string,
	chainName string,
	genesis []byte,
	subnetAuthKeysStr []string,
) (*multisig.Multisig, error) {
	fxIDs := make([]ids.ID, 0)
	subnetAuthKeys, err := address.ParseToIDs(subnetAuthKeysStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing control keys: %w", err)
	}
	subnetID, err := ids.FromString(subnetIDStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing subnet ID: %w", err)
	}
	vmID, err := ids.FromString(vmIDStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing vm ID: %w", err)
	}
	// NOT SURE ON THIS! we are modifying the client as a side effect
	// We may want to just get options for the builder, as in original deployer
	if err := client.SetSubnetAuthMultisig(subnetAuthKeys); err != nil {
		return nil, err
	}
	unsignedTx, err := client.P().Builder().NewCreateChainTx(
		subnetID,
		genesis,
		vmID,
		fxIDs,
		chainName,
	)
	if err != nil {
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	return multisig.New(&tx), nil
}
