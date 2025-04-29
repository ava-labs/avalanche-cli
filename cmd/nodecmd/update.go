// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "(ALPHA Warning) Update avalanchego or VM config for all node in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node update command suite provides a collection of commands for nodes to update
their avalanchego or VM config.

You can check the status after update by calling avalanche node status`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	// node update subnet
	cmd.AddCommand(newUpdateSubnetCmd())
	return cmd
}
