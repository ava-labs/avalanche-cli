// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package configcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"

	"github.com/spf13/cobra"
)

var app *application.Avalanche

func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Modify configuration for Avalanche-CLI",
		Long:  `Customize configuration for Avalanche-CLI`,
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	app = injectedApp
	// set user metrics collection preferences cmd
	cmd.AddCommand(newMetricsCmd())
	cmd.AddCommand(newMigrateCmd())
	cmd.AddCommand(newSingleNodeCmd())
	return cmd
}
