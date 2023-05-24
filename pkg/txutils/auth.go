// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package txutils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// get all subnet auth addresses that are required to sign a given tx
//   - get subnet control keys as string slice using P-Chain API (GetOwners)
//   - get subnet auth indices from the tx, field tx.UnsignedTx.SubnetAuth
//   - creates the string slice of required subnet auth addresses by applying
//     the indices to the control keys slice
//
// expect tx.Unsigned type to be in:
// - txs.CreateChainTx
// - txs.AddSubnetValidatorTx
// - txs.RemoveSubnetValidatorTx
func GetAuthSigners(tx *txs.Tx, network models.Network, subnetID ids.ID) ([]string, error) {
	controlKeys, _, err := GetOwners(network, subnetID)
	if err != nil {
		return nil, err
	}
	unsignedTx := tx.Unsigned
	var subnetAuth verify.Verifiable
	switch unsignedTx := unsignedTx.(type) {
	case *txs.RemoveSubnetValidatorTx:
		subnetAuth = unsignedTx.SubnetAuth
	case *txs.AddSubnetValidatorTx:
		subnetAuth = unsignedTx.SubnetAuth
	case *txs.CreateChainTx:
		subnetAuth = unsignedTx.SubnetAuth
	case *txs.TransformSubnetTx:
		subnetAuth = unsignedTx.SubnetAuth
	default:
		return nil, fmt.Errorf("unexpected unsigned tx type %T", unsignedTx)
	}
	subnetInput, ok := subnetAuth.(*secp256k1fx.Input)
	if !ok {
		return nil, fmt.Errorf("expected subnetAuth of type *secp256k1fx.Input, got %T", subnetAuth)
	}
	authSigners := []string{}
	for _, sigIndex := range subnetInput.SigIndices {
		if sigIndex >= uint32(len(controlKeys)) {
			return nil, fmt.Errorf("signer index %d exceeds number of control keys", sigIndex)
		}
		authSigners = append(authSigners, controlKeys[sigIndex])
	}
	return authSigners, nil
}

// get subnet auth addresses that did not yet signed a given tx
//   - get the string slice of auth signers for the tx (GetAuthSigners)
//   - verifies that all creds in tx.Creds, except the last one, are fully signed
//     (a cred is fully signed if all the signatures in cred.Sigs are non-empty)
//   - computes remaining signers by iterating the last cred in tx.Creds, associated to subnet auth signing
//   - for each sig in cred.Sig: if sig is empty, then add the associated auth signer address (obtained from
//     authSigners by using the index) to the remaining signers list
//
// if the tx is fully signed, returns empty slice
// expect tx.Unsigned type to be in [txs.AddSubnetValidatorTx, txs.CreateChainTx]
func GetRemainingSigners(tx *txs.Tx, network models.Network, subnetID ids.ID) ([]string, error) {
	authSigners, err := GetAuthSigners(tx, network, subnetID)
	if err != nil {
		return nil, err
	}
	emptySig := [secp256k1.SignatureLen]byte{}
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
	remainingSigners := []string{}
	for i, sig := range cred.Sigs {
		if sig == emptySig {
			remainingSigners = append(remainingSigners, authSigners[i])
		}
	}
	return remainingSigners, nil
}
