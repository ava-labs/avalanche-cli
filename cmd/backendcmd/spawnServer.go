// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package backendcmd

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

var app *application.Avalanche

// backendCmd is the command to run the backend gRPC process
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	app = injectedApp
	return &cobra.Command{
		Use:    constants.BackendCmd,
		Short:  "Run the backend server",
		Long:   "This tool requires a backend process to run; this command starts it",
		RunE:   startBackend,
		Args:   cobra.ExactArgs(0),
		Hidden: true,
	}
}

func startBackend(_ *cobra.Command, _ []string) error {
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
