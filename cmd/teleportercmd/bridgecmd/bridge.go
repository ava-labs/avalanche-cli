// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridgecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche teleporter bridge
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "Manage Teleporter Bridges",
		Long:  `The bridge command suite provides tools to deploy and manage Teleporter Bridges.`,
		RunE:  cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// contract deploy
	cmd.AddCommand(newDeployCmd())
	return cmd
}
