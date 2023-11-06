// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
)

type NetworkKind int64

const (
	Undefined NetworkKind = iota
	Mainnet
	Fuji
	Local
	Devnet
)

func (s NetworkKind) String() string {
	switch s {
	case Mainnet:
		return "Mainnet"
	case Fuji:
		return "Fuji"
	case Local:
		return "Local Network"
	case Devnet:
		return "Devnet"
	}
	return "invalid network"
}

type Network struct {
	Kind     NetworkKind
	Id       uint32
	Endpoint string
}

var (
	UndefinedNetwork = NewNetwork(Undefined, 0, "")
	LocalNetwork     = NewNetwork(Local, constants.LocalNetworkID, constants.LocalAPIEndpoint)
	DevnetNetwork    = NewNetwork(Devnet, constants.DevnetNetworkID, constants.DevnetAPIEndpoint)
	FujiNetwork      = NewNetwork(Fuji, avago_constants.FujiID, constants.FujiAPIEndpoint)
	MainnetNetwork   = NewNetwork(Mainnet, avago_constants.MainnetID, constants.MainnetAPIEndpoint)
)

func NewNetwork(kind NetworkKind, id uint32, endpoint string) Network {
	return Network{
		Kind:     kind,
		Id:       id,
		Endpoint: endpoint,
	}
}

func NetworkFromString(s string) Network {
	switch s {
	case Mainnet.String():
		return MainnetNetwork
	case Fuji.String():
		return FujiNetwork
	case Local.String():
		return LocalNetwork
	case Devnet.String():
		return DevnetNetwork
	}
	return UndefinedNetwork
}

func NetworkFromNetworkID(networkID uint32) Network {
	switch networkID {
	case avago_constants.MainnetID:
		return MainnetNetwork
	case avago_constants.FujiID:
		return FujiNetwork
	case constants.LocalNetworkID:
		return LocalNetwork
	case constants.DevnetNetworkID:
		return DevnetNetwork
	}
	return UndefinedNetwork
}
