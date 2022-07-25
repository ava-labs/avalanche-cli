// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package backendcmd

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// backendCmd is the command to run the backend gRPC process
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	app = injectedApp
	return &cobra.Command{
		Use:    "backend",
		Short:  "Run the backend server",
		Long:   "This tool requires a backend process to run; this command starts it",
		RunE:   backendController,
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}
}

func backendController(cmd *cobra.Command, args []string) error {
	if args[0] == "start" {
		return startBackend(cmd)
	}
	return fmt.Errorf("unsupported command")
}

func startBackend(_ *cobra.Command) error {
	s, err := binutils.NewGRPCServer(app.GetSnapshotsDir())
	if err != nil {
		return err
	}

	serverCtx, serverCancel := context.WithCancel(context.Background())
	errc := make(chan error)
	fmt.Println("starting server")
	go binutils.WatchServerProcess(serverCancel, errc, app.Log)
	errc <- s.Run(serverCtx)

	return nil
}
