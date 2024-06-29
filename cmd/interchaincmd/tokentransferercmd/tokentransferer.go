// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tokentransferercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche interchain tokenTransferer
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokenTransferer",
		Short: "Manage Token Transferers",
		Long:  `The tokenTransfer command suite provides tools to deploy and manage Token Transferers.`,
		RunE:  cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// tokenTransferer deploy
	cmd.AddCommand(newDeployCmd())
	return cmd
}
