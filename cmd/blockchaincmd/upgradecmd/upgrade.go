// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche blockchain upgrade
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade your Blockchains",
		Long: `The blockchain upgrade command suite provides a collection of tools for
updating your developmental and deployed Blockchains.`,
		RunE: cobrautils.CommandSuiteUsage,
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
	// subnet upgrade apply
	cmd.AddCommand(newUpgradeApplyCmd())
	return cmd
}
