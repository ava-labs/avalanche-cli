// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ava-labs/avalanche-cli/pkg/app"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/docker/docker/pkg/reexec"
	"github.com/shirou/gopsutil/process"
)

// errGRPCTimeout is a common error message if the gRPC server can't be reached
var errGRPCTimeout = errors.New("timed out trying to contact backend controller, it is most probably not running")

var latestRunDir string

func GetLatestRunDir() string {
	return latestRunDir
}

// ProcessChecker is responsible for checking if the gRPC server is running
type ProcessChecker interface {
	// IsServerProcessRunning returns true if the gRPC server is running,
	// or false if not
	IsServerProcessRunning(app *app.Avalanche) (bool, error)
}

type realProcessRunner struct{}

// NewProcessChecker creates a new process checker which can respond if the server is running
func NewProcessChecker() ProcessChecker {
	return &realProcessRunner{}
}

// NewGRPCClient hides away the details (params) of creating a gRPC server connection
func NewGRPCClient() (client.Client, error) {
	client, err := client.New(client.Config{
		LogLevel:    gRPCClientLogLevel,
		Endpoint:    gRPCServerEndpoint,
		DialTimeout: gRPCDialTimeout,
	})
	if errors.Is(err, context.DeadlineExceeded) {
		err = errGRPCTimeout
	}
	return client, err
}

// NewGRPCClient hides away the details (params) of creating a gRPC server
func NewGRPCServer(snapshotsDir string) (server.Server, error) {
	return server.New(server.Config{
		Port:                gRPCServerEndpoint,
		GwPort:              gRPCGatewayEndpoint,
		DialTimeout:         gRPCDialTimeout,
		SnapshotsDir:        snapshotsDir,
		RedirectNodesOutput: false,
	})
}

// IsServerProcessRunning returns true if the gRPC server is running,
// or false if not
func (rpr *realProcessRunner) IsServerProcessRunning(app *app.Avalanche) (bool, error) {
	pid, err := GetServerPID(app)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, err
		}
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

type runFile struct {
	Pid                int    `json:"pid"`
	GRPCserverFileName string `json:"gRPCserverFileName"`
}

func GetServerPID(app *app.Avalanche) (int, error) {
	var rf runFile
	serverRunFilePath := filepath.Join(app.GetRunDir(), constants.ServerRunFile)
	run, err := os.ReadFile(serverRunFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed reading process info file at %s: %s", serverRunFilePath, err)
	}
	if err := json.Unmarshal(run, &rf); err != nil {
		return 0, fmt.Errorf("failed unmarshalling server run file at %s: %s", serverRunFilePath, err)
	}

	if rf.Pid == 0 {
		return 0, fmt.Errorf("failed reading pid from info file at %s: %s", serverRunFilePath, err)
	}
	return rf.Pid, nil
}

// StartServerProcess starts the gRPC server as a reentrant process of this binary
// it just executes `avalanche-cli backend start`
func StartServerProcess(app app.Avalanche) error {
	thisBin := reexec.Self()

	args := []string{"backend", "start"}
	cmd := exec.Command(thisBin, args...)

	outputDirPrefix := path.Join(app.GetRunDir(), "deploy")
	outputDir, err := utils.MkDirWithTimestamp(outputDirPrefix)
	if err != nil {
		return err
	}

	// Set latest run dir
	latestRunDir = outputDir

	outputFile, err := os.Create(path.Join(outputDir, "avalanche-cli-backend"))
	if err != nil {
		return err
	}
	// TODO: should this be redirected to this app's log file instead?
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile

	if err := cmd.Start(); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Backend controller started, pid: %d, output at: %s", cmd.Process.Pid, outputFile.Name())

	rf := runFile{
		Pid:                cmd.Process.Pid,
		GRPCserverFileName: outputFile.Name(),
	}

	rfBytes, err := json.Marshal(&rf)
	if err != nil {
		return err
	}
	serverRunFilePath := filepath.Join(app.GetRunDir(), constants.ServerRunFile)
	err = os.WriteFile(serverRunFilePath, rfBytes, perms.ReadWrite)
	if err != nil {
		app.Log.Warn("could not write gRPC process info to file: %s", err)
	}
	return nil
}

// GetAsyncContext returns a timeout context with the cancel function suppressed
func GetAsyncContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
	// don't call since "start" is async
	// and the top-level context here "ctx" is passed
	// to all underlying function calls
	// just set the timeout to halt "Start" async ops
	// when the deadline is reached
	_ = cancel

	return ctx
}

func KillgRPCServerProcess(app *app.Avalanche) error {
	cli, err := NewGRPCClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := GetAsyncContext()
	_, err = cli.Stop(ctx)
	if err != nil {
		// TODO: use error type not string comparison
		if strings.Contains(err.Error(), "not bootstrapped") {
			ux.Logger.PrintToUser("No local network running")
			return nil
		}
		return fmt.Errorf("failed stopping gRPC server process: %s", err)
	}

	pid, err := GetServerPID(app)
	if err != nil {
		return fmt.Errorf("failed getting PID from run file: %s", err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("could not find process with pid %d: %s", pid, err)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed killing process with pid %d: %s", pid, err)
	}

	serverRunFilePath := filepath.Join(app.GetRunDir(), constants.ServerRunFile)
	if err := os.Remove(serverRunFilePath); err != nil {
		return fmt.Errorf("failed removing run file %s: %s", serverRunFilePath, err)
	}
	return nil
}

func WatchServerProcess(serverCancel context.CancelFunc, errc chan error, log logging.Logger) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigc:
		log.Warn("signal received: %s; closing server", sig.String())
		serverCancel()
		err := <-errc
		log.Warn("closed server: %s", err)
	case err := <-errc:
		log.Warn("server closed: %s", err)
		serverCancel()
	}
}
