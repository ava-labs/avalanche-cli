// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/ux"
)

var instructionCmd = &cobra.Command{
	Use:   "instructions [subnetName]",
	Short: "Prints the instructions for deploying a subnet on Fuji",
	Long: `The subnet instructions command prints additional instructions for taking
a subnet to Fuji. Because the beta release is still missing some
functionality, these commands will help you take your subnet to
production. As the tool matures, this command will be removed.`,

	Run:  subnetInstructions,
	Args: cobra.ExactArgs(1),
}

func subnetInstructions(cmd *cobra.Command, args []string) {
	instr := `If you can't wait to for this tool's fuji integration, you can use the subnet-cli
to deploy your subnet. Export your subnet's genesis file with

avalanche subnet describe --genesis ` + args[0] + `

Then use that genesis file to complete the instructions listed here:
https://docs.avax.network/subnets/subnet-cli`
	ux.Logger.PrintToUser(instr)
}
