// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche teleporter relayer
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relayer",
		Short: "Manage ICM relayers",
		Long: `The relayer command suite provides a collection of tools for deploying
and configuring an ICM relayers.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	cmd.AddCommand(newDeployCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	// TODO: config
	// TODO: fund
	return cmd
}
