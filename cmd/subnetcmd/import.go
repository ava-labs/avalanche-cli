// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// avalanche subnet
func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import subnets into avalanche-cli",
		Long: `The subnet command suite provides a collection of tools for developing,
manage your subnet configurations and live deployments.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	// subnet import file
	cmd.AddCommand(newImportFileCmd())
	// subnet import network
	cmd.AddCommand(newImportFromNetworkCmd())
	return cmd
}
