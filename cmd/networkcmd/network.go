// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage locally deployed subnets",
	Long: `The network command suite provides a collection of tools for managing
local subnet deployments.

When a subnet is deployed locally, it runs on a local, multi-node
Avalanche network. Deploying a subnet locally will start this network
in the background. This command suite allows you to shutdown and
restart that network.

This network currently supports multiple, concurrently deployed
subnets and will eventually support nodes with varying configurations.
Expect more functionality in future releases.`,

	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			fmt.Println(err)
		}
	},
	Args: cobra.ExactArgs(0),
}

func SetupNetworkCmd(injectedApp *application.Avalanche) *cobra.Command {
	app = injectedApp

	// network start
	networkCmd.AddCommand(startCmd)

	// network stop
	networkCmd.AddCommand(stopCmd)

	// network clean
	networkCmd.AddCommand(cleanCmd)

	// network status
	networkCmd.AddCommand(statusCmd)
	return networkCmd
}
