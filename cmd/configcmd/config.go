// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Modify configuration for Avalanche-CLI",
		Long:  `Customize configuration for Avalanche-CLI`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cobrautils.CommandSuiteUsage(cmd, args)
		},
	}
	app = injectedApp
	// set user metrics collection preferences cmd
	cmd.AddCommand(newMetricsCmd())
	cmd.AddCommand(newMigrateCmd())
	cmd.AddCommand(newSingleNodeCmd())
	cmd.AddCommand(newAuthorizeCloudAccessCmd())
	return cmd
}
