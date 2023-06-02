// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd/upgradecmd"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// avalanche subnet
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subnet",
		Short: "Create and deploy subnets",
		Long: `The subnet command suite provides a collection of tools for developing
and deploying Subnets.

To get started, use the subnet create command wizard to walk through the
configuration of your very first Subnet. Then, go ahead and deploy it
with the subnet deploy command. You can use the rest of the commands to
manage your Subnet configurations and live deployments.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
			}
		},
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
	// subnet import-running
	cmd.AddCommand(newImportFromNetworkCmd())
	// subnet VMID
	cmd.AddCommand(vmidCmd())
	// subnet removeValidator
	cmd.AddCommand(newRemoveValidatorCmd())
	// subnet elastic
	cmd.AddCommand(newElasticCmd())
	// subnet validators
	cmd.AddCommand(newValidatorsCmd())
	return cmd
}
