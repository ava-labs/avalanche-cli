// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/ux"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints the status of the local network",
	Long: `The network status command prints whether or not a local Avalanche
network is running and some basic stats about the network.`,

	RunE:         networkStatus,
	Args:         cobra.ExactArgs(0),
	SilenceUsage: true,
}

func networkStatus(cmd *cobra.Command, args []string) error {
	ux.Logger.PrintToUser("Requesting network status...")

	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return err
	}

	ctx := binutils.GetAsyncContext()
	status, err := cli.Status(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "not bootstrapped") {
			ux.Logger.PrintToUser("No local network running")
			return nil
		}
		return err
	}

	// TODO: This layout may break some screens, is there a "failsafe" way?
	if status != nil && status.ClusterInfo != nil {
		ux.Logger.PrintToUser("Network is Up. Network information:")
		ux.Logger.PrintToUser("==================================================================================================")
		ux.Logger.PrintToUser("Healthy: %t", status.ClusterInfo.Healthy)
		ux.Logger.PrintToUser("Custom VMs healthy: %t", status.ClusterInfo.CustomVmsHealthy)
		ux.Logger.PrintToUser("Number of nodes: %d", len(status.ClusterInfo.NodeNames))
		ux.Logger.PrintToUser("Number of custom VMs: %d", len(status.ClusterInfo.CustomVms))
		ux.Logger.PrintToUser("======================================== Node information ========================================")
		for n, nodeInfo := range status.ClusterInfo.NodeInfos {
			ux.Logger.PrintToUser("%s has ID %s and endpoint %s: ", n, nodeInfo.Id, nodeInfo.Uri)
		}
		ux.Logger.PrintToUser("==================================== Custom VM information =======================================")
		for _, nodeInfo := range status.ClusterInfo.NodeInfos {
			for vmID, vmInfo := range status.ClusterInfo.CustomVms {
				ux.Logger.PrintToUser("Endpoint at %s for blockchain %q: %s/ext/bc/%s", nodeInfo.Name, vmID, nodeInfo.GetUri(), vmInfo.BlockchainId)
			}
		}

	} else {
		ux.Logger.PrintToUser("No local network running")
	}

	// TODO: verbose output?
	// ux.Logger.PrintToUser(status.String())

	return nil
}
