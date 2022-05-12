// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
	fmt.Println("con")
	if args[0] == "start" {
		return startBackend(cmd)
	}
	return fmt.Errorf("Unsupported command")
}

func startBackend(cmd *cobra.Command) error {
	fmt.Println("start")
	s, err := server.New(server.Config{
		Port:        ":8097",
		GwPort:      ":8098",
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return err
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	errc := make(chan error)
	fmt.Println("starting server")
	go watchServerProcess(rootCancel, errc)
	errc <- s.Run(rootCtx)

	return nil
}

func watchServerProcess(rootCancel context.CancelFunc, errc chan error) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigc:
		zap.L().Warn("signal received; closing server", zap.String("signal", sig.String()))
		rootCancel()
		zap.L().Warn("closed server", zap.Error(<-errc))
	case err := <-errc:
		zap.L().Warn("server closed", zap.Error(err))
		rootCancel()
	}
}
