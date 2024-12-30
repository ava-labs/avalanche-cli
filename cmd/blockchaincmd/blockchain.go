// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd/upgradecmd"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche blockchain
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blockchain",
		Short: "Create and deploy blockchains",
		Long: `The blockchain command suite provides a collection of tools for developing
and deploying Blockchains.

To get started, use the blockchain create command wizard to walk through the
configuration of your very first Blockchain. Then, go ahead and deploy it
with the blockchain deploy command. You can use the rest of the commands to
manage your Blockchain configurations and live deployments.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// blockchain create
	cmd.AddCommand(newCreateCmd())
	// blockchain delete
	cmd.AddCommand(newDeleteCmd())
	// blockchain deploy
	cmd.AddCommand(newDeployCmd())
	// blockchain describe
	cmd.AddCommand(newDescribeCmd())
	// blockchain list
	cmd.AddCommand(newListCmd())
	// blockchain join
	cmd.AddCommand(newJoinCmd())
	// blockchain addValidator
	cmd.AddCommand(newAddValidatorCmd())
	// blockchain export
	cmd.AddCommand(newExportCmd())
	// blockchain import
	cmd.AddCommand(newImportCmd())
	// blockchain publish
	cmd.AddCommand(newPublishCmd())
	// blockchain upgrade
	cmd.AddCommand(upgradecmd.NewCmd(app))
	// blockchain stats
	cmd.AddCommand(newStatsCmd())
	// blockchain configure
	cmd.AddCommand(newConfigureCmd())
	// blockchain VMID
	cmd.AddCommand(vmidCmd())
	// blockchain removeValidator
	cmd.AddCommand(newRemoveValidatorCmd())
	// blockchain validators
	cmd.AddCommand(newValidatorsCmd())
	// blockchain changeOwner
	cmd.AddCommand(newChangeOwnerCmd())
	// blockchain changeWeight
	cmd.AddCommand(newChangeWeightCmd())
	return cmd
}
