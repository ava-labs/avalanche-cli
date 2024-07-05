// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/spf13/cobra"
)

// avalanche node
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Set up fuji and mainnet validator on cloud service",
		Long: `The node command suite provides a collection of tools for creating and maintaining 
validators on Avalanche Network.

To get started, use the node create command wizard to walk through the
configuration to make your node a primary validator on Avalanche public network. You can use the 
rest of the commands to maintain your node and make your node a Subnet Validator.`,
		RunE: cobrautils.CommandSuiteUsage,
	}
	app = injectedApp
	// node create
	cmd.AddCommand(newCreateCmd())
	// node validate
	cmd.AddCommand(NewValidateCmd())
	// node sync cluster --subnet subnetName
	cmd.AddCommand(newSyncCmd())
	// node destroy
	cmd.AddCommand(newDestroyCmd())
	// node status cluster
	cmd.AddCommand(newStatusCmd())
	// node list
	cmd.AddCommand(newListCmd())
	// node update
	cmd.AddCommand(newUpdateCmd())
	// node devnet
	cmd.AddCommand(newDevnetCmd())
	// node upgrade
	cmd.AddCommand(newUpgradeCmd())
	// node ssh
	cmd.AddCommand(newSSHCmd())
	// node scp
	cmd.AddCommand(newSCPCmd())
	// node whitelist
	cmd.AddCommand(newWhitelistCmd())
	// node refresh-ips
	cmd.AddCommand(newRefreshIPsCmd())
	// node loadtest
	cmd.AddCommand(NewLoadTestCmd())
	// node resize
	cmd.AddCommand(newResizeCmd())
	// node addDashboard
	cmd.AddCommand(newAddDashboardCmd())
	// node export
	cmd.AddCommand(newExportCmd())
	// node import
	cmd.AddCommand(newImportCmd())
	return cmd
}
