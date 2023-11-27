// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "(ALPHA Warning) Join Primary Network or Subnet as validator",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node validate command suite provides a collection of commands for nodes to join
the Primary Network and Subnets as validators.
If any of the commands is run before the nodes are bootstrapped on the Primary Network, the command 
will fail. You can check the bootstrap status by calling avalanche node status <clusterName>`,
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	// node validate primary cluster
	cmd.AddCommand(newValidatePrimaryCmd())
	// node validate subnet cluster subnetName
	cmd.AddCommand(newValidateSubnetCmd())
	return cmd
}
