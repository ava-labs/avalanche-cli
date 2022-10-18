// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// avalanche subnet upgrade
func newUpgrade() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade a subnet",
		Long: `The subnet upgrade command suite provides a collection of tools for updating subnets.

Run 'subnet upgrade --help' for an overview of upgrade capabilities.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	// subnet upgrade generate
	cmd.AddCommand(newUpgradeGenerateCmd())
	return cmd
}
