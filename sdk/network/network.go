// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package network

import (
	"github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/utils/constants"
)

type NetworkKind int64

const (
	Undefined NetworkKind = iota
	Mainnet
	Fuji
	Devnet
)

const (
	FujiAPIEndpoint    = "https://api.avax-test.network"
	MainnetAPIEndpoint = "https://api.avax.network"
)

type Network struct {
	Kind     NetworkKind
	ID       uint32
	Endpoint string
}

var UndefinedNetwork = Network{}

func NetworkFromNetworkID(networkID uint32) Network {
	switch networkID {
	case constants.MainnetID:
		return MainnetNetwork()
	case constants.FujiID:
		return FujiNetwork()
	}
	return UndefinedNetwork
}

func NewNetwork(kind NetworkKind, id uint32, endpoint string) Network {
	return Network{
		Kind:     kind,
		ID:       id,
		Endpoint: endpoint,
	}
}

func FujiNetwork() Network {
	return NewNetwork(Fuji, constants.FujiID, FujiAPIEndpoint)
}

func MainnetNetwork() Network {
	return NewNetwork(Mainnet, constants.MainnetID, MainnetAPIEndpoint)
}

func EndpointToNetwork(endpoint string) (*Network, error) {
	infoClient := info.NewClient(endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	networkID, err := infoClient.GetNetworkID(ctx)
	if err != nil {
		return nil, err
	}
	var kind NetworkKind
	switch networkID {
	case constants.MainnetID:
		kind = Mainnet
	case constants.FujiID:
		kind = Fuji
	default:
		kind = Devnet
	}
	network := NewNetwork(kind, networkID, endpoint)
	return &network, nil
}

func (n Network) HRP() string {
	switch n.ID {
	case constants.MainnetID:
		return constants.MainnetHRP
	case constants.FujiID:
		return constants.FujiHRP
	case constants.LocalID:
		return constants.LocalHRP
	default:
		return constants.FallbackHRP
	}
}
