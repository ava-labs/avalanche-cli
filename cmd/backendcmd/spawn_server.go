// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package backendcmd

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

var (
	app          *application.Avalanche
	serverPort   string
	gatewayPort  string
	snapshotsDir string
)

// backendCmd is the command to run the backend gRPC process
func NewCmd(injectedApp *application.Avalanche) *cobra.Command {
	app = injectedApp
	cmd := &cobra.Command{
		Use:    constants.BackendCmd,
		Short:  "Run the backend server",
		Long:   "This tool requires a backend process to run; this command starts it",
		RunE:   startBackend,
		Args:   cobrautils.ExactArgs(0),
		Hidden: true,
	}
	cmd.Flags().StringVar(&serverPort, "server-port", binutils.LocalNetworkGRPCServerPort, "server port to use")
	cmd.Flags().StringVar(&gatewayPort, "gateway-port", binutils.LocalNetworkGRPCGatewayPort, "gateway port to use")
	cmd.Flags().StringVar(&snapshotsDir, "snapshots-dir", "", "snapshots dir to use")
	return cmd
}

func startBackend(_ *cobra.Command, _ []string) error {
	if snapshotsDir == "" {
		snapshotsDir = app.GetSnapshotsDir()
	}
	s, err := binutils.NewGRPCServer(serverPort, gatewayPort, snapshotsDir)
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
