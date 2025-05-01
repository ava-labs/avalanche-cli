// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tokentransferrercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche interchain tokenTransferrer
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokenTransferrer",
		Short: "Manage Token Transferrers",
		Long:  `The tokenTransfer command suite provides tools to deploy and manage Token Transferrers.`,
		RunE:  cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// tokenTransferrer deploy
	cmd.AddCommand(NewDeployCmd())
	return cmd
}
