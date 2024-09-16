// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
)

type SubnetValidator struct {
	// Must be Ed25519 NodeID
	NodeID ids.NodeID
	// Weight of this validator used when sampling
	Weight uint64
	// Initial balance for this validator
	Balance uint64
	// [Signer] is the BLS key for this validator.
	// Note: We do not enforce that the BLS key is unique across all validators.
	// This means that validators can share a key if they so choose.
	// However, a NodeID + Subnet does uniquely map to a BLS key
	Signer signer.Signer
	// Leftover $AVAX from the [Balance] will be issued to this
	// owner once it is removed from the validator set.
	ChangeOwner fx.Owner
}
