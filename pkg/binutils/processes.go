// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/docker/docker/pkg/reexec"
	"github.com/shirou/gopsutil/process"
	"go.uber.org/zap"
)

type ProcessChecker interface {
	// IsServerProcessRunning returns true if the gRPC server is running,
	// or false if not
	IsServerProcessRunning() (bool, error)
}

type realProcessRunner struct{}

func NewProcessChecker() ProcessChecker {
	return &realProcessRunner{}
}

func NewGRPCClient() (client.Client, error) {
	return client.New(client.Config{
		LogLevel:    constants.GRPCClientLogLevel,
		Endpoint:    constants.GRPCServerEndpoint,
		DialTimeout: constants.GRPCDialTimeout,
	})
}

func NewGRPCServer() (server.Server, error) {
	return server.New(server.Config{
		Port:        constants.GRPCServerEndpoint,
		GwPort:      constants.GRPCGatewayEndpoint,
		DialTimeout: constants.GRPCDialTimeout,
	})
}

// IsServerProcessRunning returns true if the gRPC server is running,
// or false if not
func (rpr *realProcessRunner) IsServerProcessRunning() (bool, error) {
	pid, err := GetServerPID()
	if err != nil {
		return false, err
	}

	// get OS process list
	procs, err := process.Processes()
	if err != nil {
		return false, err
	}

	p32 := int32(pid)
	// iterate all processes...
	for _, p := range procs {
		if p.Pid == p32 {
			return true, nil
		}
	}
	return false, nil
}

func GetServerPID() (int, error) {
	runFile, err := os.ReadFile(constants.ServerRunFile)
	if err != nil {
		return 0, fmt.Errorf("failed reading process info file at %s: %s\n", constants.ServerRunFile, err)
	}
	str := string(runFile)
	pidIndex := strings.Index(str, "PID:")
	pidStart := pidIndex + len("PID: ")
	pidstr := str[pidStart:strings.LastIndex(str, "\n")]
	pid, err := strconv.Atoi(strings.TrimSpace(pidstr))
	if err != nil {
		return 0, fmt.Errorf("failed reading pid from info file at %s: %s\n", constants.ServerRunFile, err)
	}
	return pid, nil
}

// StartServerProcess starts the gRPC server as a reentrant process of this binary
// it just executes `avalanche-cli backend start`
func StartServerProcess(log logging.Logger) error {
	thisBin := reexec.Self()

	args := []string{"backend", "start"}
	cmd := exec.Command(thisBin, args...)
	outputFile, err := os.CreateTemp("", "avalanche-cli-backend*")
	if err != nil {
		return err
	}
	// TODO: should this be redirected to this app's log file instead?
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile

	if err := cmd.Start(); err != nil {
		return err
	}

	log.Info("Backend controller started, pid: %d, output at: %s", cmd.Process.Pid, outputFile.Name())
	content := fmt.Sprintf("gRPC server output file: %s\ngRPC server PID: %d\n", outputFile.Name(), cmd.Process.Pid)
	err = os.WriteFile(constants.ServerRunFile, []byte(content), perms.ReadWrite)
	if err != nil {
		log.Warn("could not write gRPC process info to file: %s", err)
	}
	return nil
}

func KillgRPCServerProcess() error {
	requestTimeout := 3 * time.Minute

	cli, err := NewGRPCClient()
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
		return fmt.Errorf("failed stopping gRPC server process: %s", err)
	}

	runFile, err := os.ReadFile(constants.ServerRunFile)
	if err != nil {
		return fmt.Errorf("failed reading process info file at %s: %s", constants.ServerRunFile, err)
	}
	str := string(runFile)
	pidIndex := strings.Index(str, "PID:")
	pidStart := pidIndex + len("PID: ")
	pidstr := str[pidStart:strings.LastIndex(str, "\n")]
	pid, err := strconv.Atoi(strings.TrimSpace(pidstr))
	if err != nil {
		return fmt.Errorf("failed reading pid from info file at %s: %s", constants.ServerRunFile, err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("could not find process with pid %d: %s", pid, err)
	}
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed killing process with pid %d: %s", pid, err)
	}

	return nil
}

func WatchServerProcess(serverCancel context.CancelFunc, errc chan error) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigc:
		zap.L().Warn("signal received; closing server", zap.String("signal", sig.String()))
		serverCancel()
		zap.L().Warn("closed server", zap.Error(<-errc))
	case err := <-errc:
		zap.L().Warn("server closed", zap.Error(err))
		serverCancel()
	}
}
