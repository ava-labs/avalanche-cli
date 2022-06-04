// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/spf13/cobra"
)

// backendCmd is the command to run the backend gRPC process
var backendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Run the backend server",
	Long:  "This tool requires a backend process to run; this command starts it",
	RunE:  backendController,
	Args:  cobra.ExactArgs(1),
}

func backendController(cmd *cobra.Command, args []string) error {
	if args[0] == "start" {
		return startBackend(cmd)
	}
	return fmt.Errorf("Unsupported command")
}

func startBackend(cmd *cobra.Command) error {
	s, err := binutils.NewGRPCServer(snapshotsDir)
	if err != nil {
		return err
	}

	serverCtx, serverCancel := context.WithCancel(context.Background())
	errc := make(chan error)
	fmt.Println("starting server")
	go binutils.WatchServerProcess(serverCancel, errc, log)
	errc <- s.Run(serverCtx)

	return nil
}
