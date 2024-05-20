// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

// avalanche contract deploy
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy smart contracts",
		Long:  `The deploy command suite provides deploy flows for different Smart Contracts.`,
		RunE:  cobrautils.CommandSuiteUsage,
	}
	// contract deploy
	cmd.AddCommand(newDeployBridgeCmd())
	return cmd
}
