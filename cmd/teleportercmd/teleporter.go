// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche teleporter
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teleporter",
		Short: "Interact with teleporter-enabled subnets",
		Long: `The teleporter command suite provides a collection of tools for interacting
with Teleporter-Enabled Subnets.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// teleporter msg
	cmd.AddCommand(newMsgCmd())
	// teleporter deploy
	cmd.AddCommand(newDeployCmd())
	// teleporter relayer
	cmd.AddCommand(newRelayerCmd())
	// teleporter bridge
	cmd.AddCommand(newBridgeCmd())
	return cmd
}
