// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package txutils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// get all subnet auth addresses that are required to sign a given tx
// expect tx.Unsigned type to be in [txs.AddSubnetValidatorTx, txs.CreateChainTx]
func GetAuthSigners(tx *txs.Tx, network models.Network, subnetID ids.ID) ([]string, error) {
	controlKeys, _, err := subnet.GetOwners(network, subnetID)
	if err != nil {
		return nil, err
	}
	unsignedTx := tx.Unsigned
	var subnetAuth verify.Verifiable
	switch unsignedTx := unsignedTx.(type) {
	case *txs.AddSubnetValidatorTx:
		subnetAuth = unsignedTx.SubnetAuth
	case *txs.CreateChainTx:
		subnetAuth = unsignedTx.SubnetAuth
	default:
		return nil, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	subnetInput, ok := subnetAuth.(*secp256k1fx.Input)
	if !ok {
		return nil, fmt.Errorf("expected subnetAuth of type *secp256k1fx.Input, got %T", subnetAuth)
	}
	authSigners := []string{}
	for _, addrIndex := range subnetInput.SigIndices {
		if addrIndex >= uint32(len(controlKeys)) {
			return nil, fmt.Errorf("signer index %d exceeds number of control keys", addrIndex)
		}
		authSigners = append(authSigners, controlKeys[addrIndex])
	}
	return authSigners, nil
}

// get subnet auth addresses that does not yet signed a given tx
// if the tx is fully signed, returns empty slice
// expect tx.Unsigned type to be in [txs.AddSubnetValidatorTx, txs.CreateChainTx]
func GetRemainingSigners(tx *txs.Tx, network models.Network, subnetID ids.ID) ([]string, error) {
	authSigners, err := GetAuthSigners(tx, network, subnetID)
	if err != nil {
		return nil, err
	}
	emptySig := [crypto.SECP256K1RSigLen]byte{}
	// we should have at least 1 cred for output owners and 1 cred for subnet auth
	if len(tx.Creds) < 2 {
		return nil, fmt.Errorf("expected tx.Creds of len 2, got %d", len(tx.Creds))
	}
	// signatures for output owners should be filled (all creds except last one)
	for credIndex := range tx.Creds[:len(tx.Creds)-1] {
		cred, ok := tx.Creds[credIndex].(*secp256k1fx.Credential)
		if !ok {
			return nil, fmt.Errorf("expected cred to be of type *secp256k1fx.Credential, got %T", tx.Creds[credIndex])
		}
		for i, sig := range cred.Sigs {
			if sig == emptySig {
				return nil, fmt.Errorf("expected funding sig %d of cred %d to be filled", i, credIndex)
			}
		}
	}
	// signatures for subnet auth (last cred)
	cred, ok := tx.Creds[len(tx.Creds)-1].(*secp256k1fx.Credential)
	if !ok {
		return nil, fmt.Errorf("expected cred to be of type *secp256k1fx.Credential, got %T", tx.Creds[1])
	}
	if len(cred.Sigs) != len(authSigners) {
		return nil, fmt.Errorf("expected number of cred's signatures %d to equal number of auth signers %d",
			len(cred.Sigs),
			len(authSigners),
		)
	}
	filteredAuthSigners := []string{}
	for i, sig := range cred.Sigs {
		if sig == emptySig {
			filteredAuthSigners = append(filteredAuthSigners, authSigners[i])
		}
	}
	return filteredAuthSigners, nil
}
