// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tokentransfercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche interchain tokenTransfer
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokenTransfer",
		Short: "Manage Avalanche InterChain Token Transfer (ICTT) Applications",
		Long:  `The tokenTransfer command suite provides tools to deploy and manage Avalanche InterChain Token Transfer Applications.`,
		RunE:  cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// tokenTransfer deploy
	cmd.AddCommand(newDeployCmd())
	return cmd
}
