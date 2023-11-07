// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
)

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

func (s Network) NetworkID() (uint32, error) {
	switch s {
	case Mainnet:
		return avagoconstants.MainnetID, nil
	case Fuji:
		return avagoconstants.FujiID, nil
	case Local:
		return constants.LocalNetworkID, nil
	}
	return 0, fmt.Errorf("unsupported network")
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

func NetworkFromNetworkID(networkID uint32) Network {
	switch networkID {
	case avagoconstants.MainnetID:
		return Mainnet
	case avagoconstants.FujiID:
		return Fuji
	case constants.LocalNetworkID:
		return Local
	}
	return Undefined
}
