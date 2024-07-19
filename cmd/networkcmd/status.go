// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/server"
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
	cli, err := binutils.NewGRPCClient(
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			ux.Logger.PrintToUser("No local network running")
			return nil
		}
		return err
	}
	if status != nil && status.ClusterInfo != nil {
		ux.Logger.PrintToUser("Network is Up:")
		ux.Logger.PrintToUser("  Number of Nodes: %d", len(status.ClusterInfo.NodeNames))
		ux.Logger.PrintToUser("  Number of Custom VMs: %d", len(status.ClusterInfo.CustomChains))
		ux.Logger.PrintToUser("  Network Healthy: %t", status.ClusterInfo.Healthy)
		ux.Logger.PrintToUser("  Custom VMs Healthy: %t", status.ClusterInfo.CustomChainsHealthy)
		ux.Logger.PrintToUser("")
		if err := ux.PrintLocalNetworkEndpointsInfo("", status.ClusterInfo); err != nil {
			return err
		}
	} else {
		ux.Logger.PrintToUser("No local network running")
	}

	// TODO: verbose output?
	// ux.Logger.PrintToUser(status.String())

	return nil
}
