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
	Short: "Clean up your deploy",
	Long:  `Cleans up your deploys including server processes`,

	Run:  clean,
	Args: cobra.ExactArgs(0),
}

func clean(cmd *cobra.Command, args []string) {
	log.Info("killing gRPC server process...")
	if err := binutils.KillgRPCServerProcess(); err != nil {
		log.Warn("failed killing server process: %s\n", err)
	}
	ux.PrintToUser("Process terminated.", log)
}
