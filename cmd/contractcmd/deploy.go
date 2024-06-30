// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contractcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

// avalanche contract deploy
func newDeployCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy smart contracts",
		Long: `The contract command suite provides a collection of tools for deploying
smart contracts.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// contract deploy erc20
	cmd.AddCommand(newDeployERC20Cmd())
	return cmd
}
