// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import "github.com/ava-labs/avalanchego/ids"

type Sidecar struct {
	Name         string
	Vm           VmType
	Subnet       string
	TokenName    string
	ChainID      string
	Version      string
	SubnetID     ids.ID
	BlockchainID ids.ID
}
