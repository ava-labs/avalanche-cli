// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package l1cmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

// avalanche blockchain import
func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import blockchains into avalanche-cli",
		Long: `Import blockchain configurations into avalanche-cli.

This command suite supports importing from a file created on another computer,
or importing from blockchains running public networks
(e.g. created manually or with the deprecated subnet-cli)`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	// blockchain import file
	cmd.AddCommand(newImportFileCmd())
	// blockchain import public
	cmd.AddCommand(newImportPublicCmd())
	return cmd
}
