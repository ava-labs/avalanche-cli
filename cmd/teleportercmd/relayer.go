// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

// avalanche teleporter relayer
func newRelayerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relayer",
		Short: "Install and configure relayer on localhost",
		Long: `The relayert command suite provides a collection of tools for installing
and configuring an AWM relayer on localhost.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cobrautils.CommandSuiteUsage(cmd, args)
		},
	}
	cmd.AddCommand(newPrepareRelayerServiceCmd())
	cmd.AddCommand(newAddSubnetToRelayerServiceCmd())
	cmd.AddCommand(newStopRelayerCmd())
	cmd.AddCommand(newStartRelayerCmd())
	return cmd
}
