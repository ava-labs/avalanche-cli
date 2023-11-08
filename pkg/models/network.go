// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"fmt"

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

func (nk NetworkKind) String() string {
	switch nk {
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
	ID       uint32
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
		ID:       id,
		Endpoint: endpoint,
	}
}

func NewDevnetNetwork(ip string, port int) Network {
	endpoint := fmt.Sprintf("http://%s:%d", ip, port)
	return NewNetwork(Devnet, constants.DevnetNetworkID, endpoint)
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

func (n Network) Name() string {
	return n.Kind.String()
}

func (n Network) CChainEndpoint() string {
	return fmt.Sprintf("%s/ext/bc/%s/rpc", n.Endpoint, "C")
}

func (n Network) NetworkIDFlagValue() string {
	switch n.Kind {
	case Local:
		return fmt.Sprintf("network-%d", n.ID)
	case Devnet:
		return fmt.Sprintf("network-%d", n.ID)
	case Fuji:
		return "fuji"
	case Mainnet:
		return "mainnet"
	}
	return "invalid-network"
}
