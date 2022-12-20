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
		Long: `Import subnet configurations into avalanche-cli.

This command supports importing from a file created on another computer,
or importing from subnets running public networks
(e.g. created manually or with the deprecated subnet-cli)`,
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
