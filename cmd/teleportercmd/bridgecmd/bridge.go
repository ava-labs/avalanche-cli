// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridgecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

// avalanche teleporter bridge
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bridge",
		Short: "Manage Teleporter Bridges (deprecation notice: use 'avalanche interchain tokenTransferrer')",
		Long: `The bridge command suite provides tools to deploy and manage Teleporter Bridges.

Deprecation notice: use avalanche interchain tokenTransferrer' instead`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	// contract deploy
	cmd.AddCommand(newDeployCmd())
	return cmd
}
