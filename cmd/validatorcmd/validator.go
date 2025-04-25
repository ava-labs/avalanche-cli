// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche validator
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validator",
		Short: "Manage P-Chain validator balance",
		Long: `The validator command suite provides a collection of tools for managing validator
balance on P-Chain.

Validator's balance is used to pay for continuous fee to the P-Chain. When this Balance reaches 0, 
the validator will be considered inactive and will no longer participate in validating the L1`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// validator list
	cmd.AddCommand(NewListCmd())
	// validator getBalance
	cmd.AddCommand(NewGetBalanceCmd())
	// validator increaseBalance
	cmd.AddCommand(NewIncreaseBalanceCmd())
	return cmd
}
