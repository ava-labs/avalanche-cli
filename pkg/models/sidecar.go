// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type Sidecar struct {
	Name      string
	VM        VMType
	Subnet    string
	TokenName string
	ChainID   string
	Version   string
}
