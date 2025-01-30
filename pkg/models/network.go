// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	sdkNetwork "github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/genesis"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
)

type NetworkType int64

const (
	Undefined NetworkType = iota
	Mainnet
	Fuji
	Local
	Devnet
)

const wssScheme = "wss"

func (nk NetworkType) String() string {
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
	Type        NetworkType
	ID          uint32
	Endpoint    string
	ClusterName string
}

var UndefinedNetwork = Network{}

func NewNetwork(networkType NetworkType, id uint32, endpoint string, clusterName string) Network {
	return Network{
		Type:        networkType,
		ID:          id,
		Endpoint:    endpoint,
		ClusterName: clusterName,
	}
}

func (n Network) IsUndefined() bool {
	return n.Type == Undefined
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

// ConvertClusterToNetwork converts a cluster network into a non cluster network
func ConvertClusterToNetwork(clusterNetwork Network) Network {
	if clusterNetwork.ClusterName == "" {
		return clusterNetwork
	}
	switch {
	case clusterNetwork.ID == constants.LocalNetworkID:
		return NewLocalNetwork()
	case clusterNetwork.ID == avagoconstants.FujiID:
		return NewFujiNetwork()
	case clusterNetwork.ID == avagoconstants.MainnetID:
		return NewMainnetNetwork()
	default:
		networkID := uint32(0)
		if clusterNetwork.Endpoint != "" {
			infoClient := info.NewClient(clusterNetwork.Endpoint)
			ctx, cancel := utils.GetAPIContext()
			defer cancel()
			var err error
			networkID, err = infoClient.GetNetworkID(ctx)
			if err != nil {
				return clusterNetwork
			}
			return NewDevnetNetwork(clusterNetwork.Endpoint, networkID)
		}
		return clusterNetwork
	}
}

func NewFujiNetwork() Network {
	return NewNetwork(Fuji, avagoconstants.FujiID, constants.FujiAPIEndpoint, "")
}

func NewMainnetNetwork() Network {
	return NewNetwork(Mainnet, avagoconstants.MainnetID, constants.MainnetAPIEndpoint, "")
}

func NewNetworkFromCluster(n Network, clusterName string) Network {
	return NewNetwork(n.Type, n.ID, n.Endpoint, clusterName)
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
	if n.ClusterName != "" && n.Type == Devnet {
		return "Cluster " + n.ClusterName
	}
	name := n.Type.String()
	if n.Type == Devnet {
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
	switch n.Type {
	case Fuji:
		scheme = wssScheme
	case Mainnet:
		scheme = wssScheme
	}
	return fmt.Sprintf("%s://%s/ext/bc/%s/ws", scheme, trimmedURI, blockchainID)
}

func (n Network) NetworkIDFlagValue() string {
	switch n.Type {
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
	switch n.Type {
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
		n.Type = Local
		n.ID = constants.LocalNetworkID
		n.Endpoint = constants.LocalAPIEndpoint
	}
}

// Equals checks the underlying fields Type and Endpoint
func (n *Network) Equals(n2 Network) bool {
	return n.Type == n2.Type && n.Endpoint == n2.Endpoint
}

// Context for bootstrapping a partial synced Node
func (n *Network) BootstrappingContext() (context.Context, context.CancelFunc) {
	timeout := constants.ANRRequestTimeout
	switch n.Type {
	case Fuji:
		timeout = constants.FujiBootstrapTimeout
	case Mainnet:
		timeout = constants.MainnetBootstrapTimeout
	}
	return context.WithTimeout(context.Background(), timeout)
}

func (n Network) SDKNetwork() sdkNetwork.Network {
	switch n.Type {
	case Fuji:
		return sdkNetwork.FujiNetwork()
	case Mainnet:
		return sdkNetwork.MainnetNetwork()
	case Local:
		return sdkNetwork.NewNetwork(sdkNetwork.Devnet, n.ID, n.Endpoint)
	case Devnet:
		return sdkNetwork.NewNetwork(sdkNetwork.Devnet, n.ID, n.Endpoint)
	}
	return sdkNetwork.UndefinedNetwork
}

// GetNetworkFromCluster gets the network that a cluster is on
func GetNetworkFromCluster(clusterConfig ClusterConfig) Network {
	network := clusterConfig.Network
	switch {
	case network.ID == constants.LocalNetworkID:
		return NewLocalNetwork()
	case network.ID == avagoconstants.FujiID:
		return NewFujiNetwork()
	case network.ID == avagoconstants.MainnetID:
		return NewMainnetNetwork()
	default:
		return network
	}
}

func GetWSEndpoint(endpoint string, blockchainID string) string {
	return NewDevnetNetwork(endpoint, 0).BlockchainWSEndpoint(blockchainID)
}

func GetRPCEndpoint(endpoint string, blockchainID string) string {
	return NewDevnetNetwork(endpoint, 0).BlockchainEndpoint(blockchainID)
}
