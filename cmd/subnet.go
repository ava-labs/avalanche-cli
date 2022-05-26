// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// subnetCmd represents the subnet command
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
