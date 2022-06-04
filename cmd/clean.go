// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/ux"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Stop the running local network and delete state",
	Long: `The network clean command shuts down your local, multi-node network. All
the deployed subnets will shutdown and delete their state. The network
may be started again by deploying a new subnet configuration.`,

	Run:  clean,
	Args: cobra.ExactArgs(0),
}

func clean(cmd *cobra.Command, args []string) {
	log.Info("killing gRPC server process...")
	if err := binutils.KillgRPCServerProcess(); err != nil {
		log.Warn("failed killing server process: %s\n", err)
		ux.Logger.PrintToUser("Unable to terminate process. Most probably has already been terminated.")
	} else {
		ux.Logger.PrintToUser("Process terminated.")
	}
}
