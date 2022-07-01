// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

func NewSubnetCmd(injectedApp *application.Avalanche) *cobra.Command {
	app = injectedApp

	// subnet create
	subnetCmd.AddCommand(newCreateCmd())

	// subnet delete
	subnetCmd.AddCommand(newDeleteCmd())

	// subnet deploy
	subnetCmd.AddCommand(newDeployCmd())

	// subnet describe
	subnetCmd.AddCommand(newDescribeCmd())

	// subnet list
	subnetCmd.AddCommand(newListCmd())
	return subnetCmd
}

// avalanche subnet
var subnetCmd = &cobra.Command{
	Use:   "subnet",
	Short: "Create and deploy subnets",
	Long: `The subnet command suite provides a collection of tools for developing
and deploying subnets.

To get started, use the subnet create command wizard to walk through the
configuration of your very first subnet. Then, go ahead and deploy it
with the subnet deploy command. You can use the rest of the commands to
manage your subnet configurations.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			fmt.Println(err)
		}
	},
}
