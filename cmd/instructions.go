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
	ux.Logger.PrintToUser("Unimplemented")
}
