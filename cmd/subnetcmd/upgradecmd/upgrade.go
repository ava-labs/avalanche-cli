// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche subnet vm
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade your subnets",
		Long: `The subnet upgrade command suite provides a collection of tools for
updating your developmental and deployed subnets.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
	}
	app = injectedApp
	// subnet upgrade vm
	cmd.AddCommand(newUpgradeVMCmd())
	// subnet upgrade generate
	cmd.AddCommand(newUpgradeGenerateCmd())
	// subnet upgrade import
	cmd.AddCommand(newUpgradeImportCmd())
	// subnet upgrade export
	cmd.AddCommand(newUpgradeExportCmd())
	// subnet upgrade print
	cmd.AddCommand(newUpgradePrintCmd())
	// subnet upgrade
	cmd.AddCommand(newUpgradeInstallCmd())
	return cmd
}
