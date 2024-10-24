// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche contract
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract",
		Short: "Manage smart contracts",
		Long: `The contract command suite provides a collection of tools for deploying
and interacting with smart contracts.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// contract deploy
	cmd.AddCommand(newDeployCmd())
	// contract initpoamanager
	cmd.AddCommand(newInitPOAManagerCmd())
	return cmd
}
