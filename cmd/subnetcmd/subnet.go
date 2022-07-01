// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

func SetupSubnetCmd(injectedApp *application.Avalanche) *cobra.Command {
	app = injectedApp

	// subnet create
	subnetCmd.AddCommand(createCmd)
	createCmd.Flags().StringVar(&filename, "file", "", "file path of genesis to use instead of the wizard")
	createCmd.Flags().BoolVar(&useSubnetEvm, "evm", false, "use the SubnetEVM as the base template")
	createCmd.Flags().BoolVar(&useCustom, "custom", false, "use a custom VM template")
	createCmd.Flags().BoolVarP(&forceCreate, forceFlag, "f", false, "overwrite the existing configuration if one exists")

	// subnet delete
	subnetCmd.AddCommand(deleteCmd)

	// subnet deploy
	subnetCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&deployLocal, "local", "l", false, "deploy to a local network")

	// subnet describe
	subnetCmd.AddCommand(describeCmd)
	describeCmd.Flags().BoolVarP(
		&printGenesisOnly,
		"genesis",
		"g",
		false,
		"Print the genesis to the console directly instead of the summary",
	)

	// subnet list
	subnetCmd.AddCommand(listCmd)
	return subnetCmd
}

// avalanche subnet
var subnetCmd = &cobra.Command{
	Use:   "subnet",
	Short: "Create and deploy subnets",
	Long: `The subnet command suite provides a collection of tools for developing
and deploying subnets.

To get started, use the subnet create command wizard to walk through the
configuration of your very first subnet. Then, go ahead and deploy it
with the subnet deploy command. You can use the rest of the commands to
manage your subnet configurations.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			fmt.Println(err)
		}
	},
}
