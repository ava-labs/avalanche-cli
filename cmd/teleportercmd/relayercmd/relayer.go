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
		Short: "Install and configure relayer on localhost",
		Long: `The relayert command suite provides a collection of tools for installing
and configuring an AWM relayer on localhost.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	cmd.AddCommand(newPrepareServiceCmd())
	cmd.AddCommand(newAddSubnetToServiceCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newLogsCmd())
	cmd.AddCommand(newDeployCmd())
	return cmd
}
