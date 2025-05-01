// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
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
	// blockchain upgrade vm
	cmd.AddCommand(newUpgradeVMCmd())
	// blockchain upgrade generate
	cmd.AddCommand(newUpgradeGenerateCmd())
	// blockchain upgrade import
	cmd.AddCommand(newUpgradeImportCmd())
	// blockchain upgrade export
	cmd.AddCommand(newUpgradeExportCmd())
	// blockchain upgrade print
	cmd.AddCommand(newUpgradePrintCmd())
	// blockchain upgrade apply
	cmd.AddCommand(newUpgradeApplyCmd())
	return cmd
}
