// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type SubnetValidator struct {
	NodeID string `json:"NodeID"`

	Weight uint64 `json:"Weight"`

	Balance uint64 `json:"Balance"`

	BLSPublicKey string `json:"BLSPublicKey"`

	BLSProofOfPossession string `json:"BLSProofOfPossession"`

	ChangeOwnerAddr string `json:"ChangeOwnerAddr"`

	ValidationID string `json:"ValidationID"`
}
