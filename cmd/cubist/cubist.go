// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cubist

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	app = injectedApp

	cmd := &cobra.Command{
		Use:   "cubist",
		Short: "Create and manage testnet signing keys",
		Long: `The key command suite provides a collection of tools for creating and managing
signing keys. You can use these keys to deploy Subnets to the Fuji Testnet,
but these keys are NOT suitable to use in production environments. DO NOT use
these keys on Mainnet.

To get started, use the key create command.`,
		RunE: cobrautils.CommandSuiteUsage,
	}

	// avalanche key create
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newTestCmd())

	return cmd
}
