// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Prints the status of the local network",
		Long: `The network status command prints whether or not a local Avalanche
network is running and some basic stats about the network.`,

		RunE: networkStatus,
		Args: cobrautils.ExactArgs(0),
	}
}

func networkStatus(*cobra.Command, []string) error {
	network, err := localnet.GetLocalNetwork(app)
	if err != nil {
		return err
	}
	clusters, err := localnet.GetLocalNetworkRunningClusters(app)
	if err != nil {
		return err
	}
	nodesCount := len(network.Nodes)
	for _, clusterName := range clusters {
		network, err := localnet.GetLocalCluster(app, clusterName)
		if err != nil {
			return err
		}
		nodesCount += len(network.Nodes)
	}
	blockchains, err := localnet.GetLocalNetworkBlockchainInfo(app)
	if err != nil {
		return err
	}
	pChainBootstrapped, blockchainsBootstrapped, err := localnet.LocalNetworkHealth(app, ux.Logger.PrintToUser)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Network is Up:")
	ux.Logger.PrintToUser("  Number of Nodes: %d", nodesCount)
	ux.Logger.PrintToUser("  Number of Blockchains: %d", len(blockchains))
	ux.Logger.PrintToUser("  Network Healthy: %t", pChainBootstrapped)
	ux.Logger.PrintToUser("  Blockchains Healthy: %t", blockchainsBootstrapped)
	ux.Logger.PrintToUser("")
	if err := localnet.PrintEndpoints(app, ux.Logger.PrintToUser, ""); err != nil {
		return err
	}

	return nil
}
