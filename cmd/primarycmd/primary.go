// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package primarycmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche primary
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "primary",
		Short: "Interact with the Primary Network",
		Long: `The primary command suite provides a collection of tools for interacting with the
Primary Network`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// primary addValidator
	cmd.AddCommand(newAddValidatorCmd())
	// primary describe
	cmd.AddCommand(newDescribeCmd())
	return cmd
}
