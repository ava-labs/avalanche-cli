// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/genesis"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
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
	Kind        NetworkKind
	ID          uint32
	Endpoint    string
	ClusterName string
}

var UndefinedNetwork = Network{}

func NewNetwork(kind NetworkKind, id uint32, endpoint string, clusterName string) Network {
	return Network{
		Kind:        kind,
		ID:          id,
		Endpoint:    endpoint,
		ClusterName: clusterName,
	}
}

func (n Network) IsUndefined() bool {
	return n.Kind == Undefined
}

func NewLocalNetwork() Network {
	return NewNetwork(Local, constants.LocalNetworkID, constants.LocalAPIEndpoint, "")
}

func NewDevnetNetwork(endpoint string, id uint32) Network {
	if endpoint == "" {
		endpoint = constants.DevnetAPIEndpoint
	}
	if id == 0 {
		id = constants.DevnetNetworkID
	}
	return NewNetwork(Devnet, id, endpoint, "")
}

func NewFujiNetwork() Network {
	return NewNetwork(Fuji, avagoconstants.FujiID, constants.FujiAPIEndpoint, "")
}

func NewMainnetNetwork() Network {
	return NewNetwork(Mainnet, avagoconstants.MainnetID, constants.MainnetAPIEndpoint, "")
}

func NewNetworkFromCluster(n Network, clusterName string) Network {
	return NewNetwork(n.Kind, n.ID, n.Endpoint, clusterName)
}

func NetworkFromNetworkID(networkID uint32) Network {
	switch networkID {
	case avagoconstants.MainnetID:
		return NewMainnetNetwork()
	case avagoconstants.FujiID:
		return NewFujiNetwork()
	case constants.LocalNetworkID:
		return NewLocalNetwork()
	}
	return UndefinedNetwork
}

func (n Network) StandardPublicEndpoint() bool {
	return n.Endpoint == constants.FujiAPIEndpoint || n.Endpoint == constants.MainnetAPIEndpoint
}

func (n Network) Name() string {
	if n.ClusterName != "" && n.Kind == Devnet {
		return "Cluster " + n.ClusterName
	}
	name := n.Kind.String()
	if n.Kind == Devnet {
		name += " " + n.Endpoint
	}
	return name
}

func (n Network) CChainEndpoint() string {
	return n.BlockchainEndpoint("C")
}

func (n Network) CChainWSEndpoint() string {
	return n.BlockchainWSEndpoint("C")
}

func (n Network) BlockchainEndpoint(blockchainID string) string {
	return fmt.Sprintf("%s/ext/bc/%s/rpc", n.Endpoint, blockchainID)
}

func (n Network) BlockchainWSEndpoint(blockchainID string) string {
	trimmedURI := n.Endpoint
	trimmedURI = strings.TrimPrefix(trimmedURI, "http://")
	trimmedURI = strings.TrimPrefix(trimmedURI, "https://")
	scheme := "ws"
	switch n.Kind {
	case Fuji:
		scheme = "wss"
	case Mainnet:
		scheme = "wss"
	}
	return fmt.Sprintf("%s://%s/ext/bc/%s/ws", scheme, trimmedURI, blockchainID)
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

func (n Network) GenesisParams() *genesis.Params {
	switch n.Kind {
	case Local:
		return &genesis.LocalParams
	case Devnet:
		return &genesis.LocalParams
	case Fuji:
		return &genesis.FujiParams
	case Mainnet:
		return &genesis.MainnetParams
	}
	return nil
}

func (n *Network) HandlePublicNetworkSimulation() {
	// used in E2E to simulate public network execution paths on a local network
	if os.Getenv(constants.SimulatePublicNetwork) != "" {
		n.Kind = Local
		n.ID = constants.LocalNetworkID
		n.Endpoint = constants.LocalAPIEndpoint
	}
}
