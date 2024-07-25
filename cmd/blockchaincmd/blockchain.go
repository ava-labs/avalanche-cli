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

// avalanche subnet
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
	// subnet create
	cmd.AddCommand(newCreateCmd())
	// subnet delete
	cmd.AddCommand(newDeleteCmd())
	// subnet deploy
	cmd.AddCommand(newDeployCmd())
	// subnet describe
	cmd.AddCommand(newDescribeCmd())
	// subnet list
	cmd.AddCommand(newListCmd())
	// subnet join
	cmd.AddCommand(newJoinCmd())
	// subnet addValidator
	cmd.AddCommand(newAddValidatorCmd())
	// subnet export
	cmd.AddCommand(newExportCmd())
	// subnet import
	cmd.AddCommand(newImportCmd())
	// subnet publish
	cmd.AddCommand(newPublishCmd())
	// subnet upgrade
	cmd.AddCommand(upgradecmd.NewCmd(app))
	// subnet stats
	cmd.AddCommand(newStatsCmd())
	// subnet configure
	cmd.AddCommand(newConfigureCmd())
	// subnet VMID
	cmd.AddCommand(vmidCmd())
	// subnet removeValidator
	cmd.AddCommand(newRemoveValidatorCmd())
	// subnet elastic
	cmd.AddCommand(newElasticCmd())
	// subnet validators
	cmd.AddCommand(newValidatorsCmd())
	// subnet addPermissionlessDelegator
	cmd.AddCommand(newAddPermissionlessDelegatorCmd())
	// subnet changeOwner
	cmd.AddCommand(newChangeOwnerCmd())
	return cmd
}
