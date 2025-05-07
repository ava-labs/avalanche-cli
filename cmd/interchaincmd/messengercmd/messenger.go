// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package messengercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche interchain messenger
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messenger",
		Short: "Interact with ICM messenger contracts",
		Long: `The messenger command suite provides a collection of tools for interacting
with ICM messenger contracts.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// interchain messenger sendMsg
	cmd.AddCommand(NewSendMsgCmd())
	// interchain messenger deploy
	cmd.AddCommand(NewDeployCmd())
	return cmd
}
