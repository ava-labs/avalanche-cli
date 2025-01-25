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
	blockchains, err := localnet.GetLocalNetworkBlockchainInfo(app)
	if err != nil {
		return err
	}
	pChainBootstrapped, blockchainsBootstrapped, err := localnet.LocalNetworkHealth(app)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Network is Up:")
	ux.Logger.PrintToUser("  Number of Nodes: %d", len(network.Nodes))
	ux.Logger.PrintToUser("  Number of Custom VMs: %d", len(blockchains))
	ux.Logger.PrintToUser("  Network Healthy: %t", pChainBootstrapped)
	ux.Logger.PrintToUser("  Custom VMs Healthy: %t", blockchainsBootstrapped)
	ux.Logger.PrintToUser("")
	if err := localnet.PrintEndpoints(app, ux.Logger.PrintToUser, ""); err != nil {
		return err
	}

	return nil
}
