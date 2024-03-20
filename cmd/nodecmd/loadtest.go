// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewLoadTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loadtest",
		Short: "(ALPHA Warning) Load test suite for an existing subnet on an existing cloud cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node loadtest command suite starts and stops a load test for an existing devnet cluster.`,
		Run: func(cmd *cobra.Command, _ []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	// node loadtest start cluster subnetName
	cmd.AddCommand(newLoadTestStartCmd())
	// node loadtest stop cluster
	cmd.AddCommand(newLoadTestStopCmd())
	return cmd
}
