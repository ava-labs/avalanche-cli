// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import "github.com/ava-labs/avalanchego/ids"

type NetworkData struct {
	SubnetID     ids.ID
	BlockchainID ids.ID
	Threshold    uint32
}

type Sidecar struct {
	Name            string
	VM              VMType
	VMVersion       string
	Subnet          string
	TokenName       string
	ChainID         string
	Version         string
	Networks        map[string]NetworkData
	ImportedFromAPM bool
	ImportedVMID    string
}
