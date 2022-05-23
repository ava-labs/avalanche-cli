// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/ux"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running local network and preserve state",
	Long: `The network stop command shuts down your local, multi-node network. All
the deployed subnets will shutdown gracefully and save their state. The
network may be started again with network start.`,

	Run:  stopNetwork,
	Args: cobra.ExactArgs(0),
}

func stopNetwork(cmd *cobra.Command, args []string) {
	ux.Logger.PrintToUser("Unimplemented")
}
