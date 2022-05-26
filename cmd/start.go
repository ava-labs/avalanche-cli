// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/ux"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts a stopped local network",
	Long: `The network start command starts a local, multi-node Avalanche network
on your machine. Any subnets that have been previously deployed will be
resumed with their old state. The command may fail if the local network
is already running or if no subnets have been deployed.`,

	Run:  startNetwork,
	Args: cobra.ExactArgs(0),
}

func startNetwork(cmd *cobra.Command, args []string) {
	ux.Logger.PrintToUser("Unimplemented")
}
