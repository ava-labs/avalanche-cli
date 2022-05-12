// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/spf13/cobra"
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
	if err := killgRPCServerProcess(); err != nil {
		log.Warn("failed killing server process: %s\n", err)
	}
	log.Info("process terminated.")
}

func killgRPCServerProcess() error {
	requestTimeout := 3 * time.Minute

	cli, err := client.New(client.Config{
		LogLevel:    gRPCClientLogLevel,
		Endpoint:    gRPCServerEndpoint,
		DialTimeout: gRPCDialTimeout,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	// don't call since "start" is async
	// and the top-level context here "ctx" is passed
	// to all underlying function calls
	// just set the timeout to halt "Start" async ops
	// when the deadline is reached
	_ = cancel

	_, err = cli.Stop(ctx)
	if err != nil {
		log.Error("failed stopping gRPC server process: %s\n", err)
	}

	runFile, err := os.ReadFile(serverRun)
	if err != nil {
		return fmt.Errorf("failed reading process info file at %s: %s\n", serverRun, err)
	}
	str := string(runFile)
	pidIndex := strings.Index(str, "PID:")
	pidStart := pidIndex + len("PID: ")
	pidstr := str[pidStart:strings.LastIndex(str, "\n")]
	pid, err := strconv.Atoi(strings.TrimSpace(pidstr))
	if err != nil {
		return fmt.Errorf("failed reading pid from info file at %s: %s\n", serverRun, err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("could not find process with pid %d: %s\n", pid, err)
	}
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed killing process with pid %d: %s\n", pid, err)
	}

	return nil
}
