// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import "github.com/ava-labs/avalanchego/utils/constants"

type Network int64

const (
	Undefined Network = iota
	Mainnet
	Fuji
	Local
)

func (s Network) String() string {
	switch s {
	case Mainnet:
		return "Mainnet"
	case Fuji:
		return "Fuji"
	case Local:
		return "Local Network"
	}
	return "Unknown Network"
}

func (s Network) NetworkID() uint32 {
	switch s {
	case Mainnet:
		return constants.MainnetID
	case Fuji:
		return constants.FujiID
	case Local:
		return 0
	}
	return 0
}

func NetworkFromString(s string) Network {
	switch s {
	case Mainnet.String():
		return Mainnet
	case Fuji.String():
		return Fuji
	case Local.String():
		return Local
	}
	return Undefined
}
