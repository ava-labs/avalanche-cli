// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche subnet vm
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transaction",
		Short: "Sign and execute specific transactions",
		Long:  `The transaction command suite provides all of the utilities required to sign multisig transactions.`,
		RunE:  cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// subnet upgrade vm
	cmd.AddCommand(newTransactionSignCmd())
	// subnet upgrade generate
	cmd.AddCommand(newTransactionCommitCmd())
	return cmd
}
