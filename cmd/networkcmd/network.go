// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	app = injectedApp
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage locally deployed subnets",
		Long: `The network command suite provides a collection of tools for managing local Subnet
deployments.

When you deploy a Subnet locally, it runs on a local, multi-node Avalanche network. The
subnet deploy command starts this network in the background. This command suite allows you
to shutdown, restart, and clear that network.

This network currently supports multiple, concurrently deployed Subnets.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cobrautils.CommandSuiteUsage(cmd, args)
		},
		Args: cobrautils.ExactArgs(0),
	}
	// network start
	cmd.AddCommand(newStartCmd())
	// network stop
	cmd.AddCommand(newStopCmd())
	// network clean
	cmd.AddCommand(newCleanCmd())
	// network status
	cmd.AddCommand(newStatusCmd())
	return cmd
}
