// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package signatureAggregatorCmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche interchain signatureAggregator
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signatureAggregator",
		Short: "Manage ICM signature aggregator",
		Long: `The signature aggregator command suite provides a collection of tools for deploying
and configuring ICM signature aggregator.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	cmd.AddCommand(newStopCmd())
	return cmd
}
