// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/ux"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Prints the status of the local network",
	Long: `The network status command prints whether or not a local Avalanche
network is running and some basic stats about the network.`,

	Run:  networkStatus,
	Args: cobra.ExactArgs(0),
}

func networkStatus(cmd *cobra.Command, args []string) {
	ux.Logger.PrintToUser("Unimplemented")
}
