// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

// avalanche teleporter bridge
func newBridgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "Manage Teleporter Bridges",
		Long:  `The bridge command suite provides tools to deploy and manage Teleporter Bridges.`,
		RunE:  cobrautils.CommandSuiteUsage,
	}
	// contract deploy
	cmd.AddCommand(newBridgeDeployCmd())
	return cmd
}
