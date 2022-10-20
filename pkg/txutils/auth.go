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
	}
	subnetInput, ok := subnetAuth.(*secp256k1fx.Input)
	if !ok {
		return nil, fmt.Errorf("expected subnetAuth of type *secp256k1fx.Input, got %T", subnetAuth)
	}
	authSigners := []string{}
	for _, addrIndex := range subnetInput.SigIndices {
		if addrIndex >= uint32(len(controlKeys)) {
			return nil, fmt.Errorf("addrIndex %d > len(controlKeys) %d", addrIndex, len(controlKeys))
		}
		authSigners = append(authSigners, controlKeys[addrIndex])
	}
	return authSigners, nil
}

func GetRemainingSigners(tx *txs.Tx, network models.Network, subnetID ids.ID) ([]string, error) {
	authSigners, err := GetAuthSigners(tx, network, subnetID)
	if err != nil {
		return nil, err
	}
	emptySig := [crypto.SECP256K1RSigLen]byte{}
	// we should have 1 cred for funding and 1 cred for subnet auth
	if len(tx.Creds) != 2 {
		return nil, fmt.Errorf("expected tx.Creds of len 2, got %d", len(tx.Creds))
	}
	// signatures for funding address should be filled
	cred, ok := tx.Creds[0].(*secp256k1fx.Credential)
	if !ok {
		return nil, fmt.Errorf("expected cred to be of type *secp256k1fx.Credential, got %T", tx.Creds[0])
	}
	for i, sig := range cred.Sigs {
		if sig == emptySig {
			return nil, fmt.Errorf("expected funding sig %d to be filled", i)
		}
	}
	// signatures for subnet auth
	cred, ok = tx.Creds[1].(*secp256k1fx.Credential)
	if !ok {
		return nil, fmt.Errorf("expected cred to be of type *secp256k1fx.Credential, got %T", tx.Creds[1])
	}
	filteredAuthSigners := []string{}
	for i, sig := range cred.Sigs {
		if sig == emptySig {
			filteredAuthSigners = append(filteredAuthSigners, authSigners[i])
		}
	}
	return filteredAuthSigners, nil
}
