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

// CreateChainTxParams contains all parameters needed to create a CreateChainTx
type CreateChainTxParams struct {
	// SubnetID is the ID of the subnet where the chain will be created
	SubnetID string
	// VMID is the ID of the virtual machine to be associated to the chain
	VMID string
	// ChainName is the chain name
	ChainName string
	// Genesis details the contents of the genesis block
	Genesis []byte
	// SubnetAuthKeys are the subset of the subnet control keys used to
	// authorize the operation
	SubnetAuthKeys []string
}

// NewCreateChainTx creates uncommitted CreateChainTx
// keychain in wallet will be used to build the transaction
func NewCreateChainTx(
	client wallet.Wallet,
	createChainTxParams CreateChainTxParams,
) (*multisig.Multisig, error) {
	fxIDs := make([]ids.ID, 0)
	subnetAuthKeys, err := address.ParseToIDs(createChainTxParams.SubnetAuthKeys)
	if err != nil {
		return nil, fmt.Errorf("error parsing control keys: %w", err)
	}
	subnetID, err := ids.FromString(createChainTxParams.SubnetID)
	if err != nil {
		return nil, fmt.Errorf("error parsing subnet ID: %w", err)
	}
	vmID, err := ids.FromString(createChainTxParams.VMID)
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
		createChainTxParams.Genesis,
		vmID,
		fxIDs,
		createChainTxParams.ChainName,
	)
	if err != nil {
		return nil, fmt.Errorf("error building tx: %w", err)
	}
	tx := txs.Tx{Unsigned: unsignedTx}
	return multisig.New(&tx), nil
}
