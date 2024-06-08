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
	"syscall"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/server"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/docker/docker/pkg/reexec"
	"github.com/shirou/gopsutil/process"
	"go.uber.org/zap"
)

// ErrGRPCTimeout is a common error message if the gRPC server can't be reached
var ErrGRPCTimeout = errors.New("timed out trying to contact backend controller, it is most probably not running")

// ProcessChecker is responsible for checking if the gRPC server is running
type ProcessChecker interface {
	// IsServerProcessRunning returns true if the gRPC server is running,
	// or false if not
	IsServerProcessRunning(app *application.Avalanche) (bool, error)
}

type realProcessRunner struct{}

// NewProcessChecker creates a new process checker which can respond if the server is running
func NewProcessChecker() ProcessChecker {
	return &realProcessRunner{}
}

type GRPCClientOp struct {
	avoidRPCVersionCheck bool
	dialTimeout          time.Duration
}

type GRPCClientOpOption func(*GRPCClientOp)

func (op *GRPCClientOp) applyOpts(opts []GRPCClientOpOption) {
	for _, opt := range opts {
		opt(op)
	}
}

func WithAvoidRPCVersionCheck(avoidRPCVersionCheck bool) GRPCClientOpOption {
	return func(op *GRPCClientOp) {
		op.avoidRPCVersionCheck = avoidRPCVersionCheck
	}
}

func WithDialTimeout(dialTimeout time.Duration) GRPCClientOpOption {
	return func(op *GRPCClientOp) {
		op.dialTimeout = dialTimeout
	}
}

// NewGRPCClient hides away the details (params) of creating a gRPC server connection
func NewGRPCClient(opts ...GRPCClientOpOption) (client.Client, error) {
	op := GRPCClientOp{
		dialTimeout: gRPCDialTimeout,
	}
	op.applyOpts(opts)
	logLevel, err := logging.ToLevel(gRPCClientLogLevel)
	if err != nil {
		return nil, err
	}
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logLevel,
		LogLevel:     logging.Off,
	})
	log, err := logFactory.Make("grpc-client")
	if err != nil {
		return nil, err
	}
	client, err := client.New(client.Config{
		Endpoint:    gRPCServerEndpoint,
		DialTimeout: op.dialTimeout,
	}, log)
	if errors.Is(err, context.DeadlineExceeded) {
		err = ErrGRPCTimeout
	}
	if client != nil && !op.avoidRPCVersionCheck {
		ctx, cancel := utils.GetAPIContext()
		defer cancel()
		rpcVersion, err := client.RPCVersion(ctx)
		if err != nil {
			return nil, err
		}
		// obtained using server API
		serverVersion := rpcVersion.Version
		// obtained from ANR source code
		clientVersion := server.RPCVersion
		if serverVersion != clientVersion {
			return nil, fmt.Errorf("trying to connect to a backend controller that uses a different RPC version (%d) than the CLI client (%d). Use 'network stop' to stop the controller and then restart the operation",
				serverVersion,
				clientVersion)
		}
	}
	return client, err
}

// NewGRPCServer hides away the details (params) of creating a gRPC server
func NewGRPCServer(snapshotsDir string) (server.Server, error) {
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logging.Info,
		LogLevel:     logging.Off,
	})
	log, err := logFactory.Make("grpc-server")
	if err != nil {
		return nil, err
	}
	return server.New(server.Config{
		Port:                gRPCServerPort,
		GwPort:              gRPCGatewayPort,
		DialTimeout:         gRPCDialTimeout,
		SnapshotsDir:        snapshotsDir,
		RedirectNodesOutput: false,
		LogLevel:            logging.Info,
	}, log)
}

// IsServerProcessRunning returns true if the gRPC server is running,
// or false if not
func (*realProcessRunner) IsServerProcessRunning(app *application.Avalanche) (bool, error) {
	pid, err := GetServerPID(app)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		return false, nil
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

func GetBackendLogFile(app *application.Avalanche) (string, error) {
	var rf runFile
	serverRunFilePath := app.GetRunFile()
	run, err := os.ReadFile(serverRunFilePath)
	if err != nil {
		return "", fmt.Errorf("failed reading process info file at %s: %w", serverRunFilePath, err)
	}
	if err := json.Unmarshal(run, &rf); err != nil {
		return "", fmt.Errorf("failed unmarshalling server run file at %s: %w", serverRunFilePath, err)
	}

	return rf.GRPCserverFileName, nil
}

func GetServerPID(app *application.Avalanche) (int, error) {
	var rf runFile
	serverRunFilePath := app.GetRunFile()
	run, err := os.ReadFile(serverRunFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed reading process info file at %s: %w", serverRunFilePath, err)
	}
	if err := json.Unmarshal(run, &rf); err != nil {
		return 0, fmt.Errorf("failed unmarshalling server run file at %s: %w", serverRunFilePath, err)
	}

	if rf.Pid == 0 {
		return 0, fmt.Errorf("failed reading pid from info file at %s: %w", serverRunFilePath, err)
	}
	return rf.Pid, nil
}

// StartServerProcess starts the gRPC server as a reentrant process of this binary
// it just executes `avalanche-cli backend start`
func StartServerProcess(app *application.Avalanche) error {
	thisBin := reexec.Self()

	args := []string{constants.BackendCmd}
	cmd := exec.Command(thisBin, args...)

	outputDirPrefix := path.Join(app.GetRunDir(), "server")
	outputDir, err := anrutils.MkDirWithTimestamp(outputDirPrefix)
	if err != nil {
		return err
	}

	outputFile, err := os.Create(path.Join(outputDir, "avalanche-cli-backend.log"))
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

	if err := os.WriteFile(app.GetRunFile(), rfBytes, perms.ReadWrite); err != nil {
		app.Log.Warn("could not write gRPC process info to file", zap.Error(err))
	}
	return nil
}

func KillgRPCServerProcess(app *application.Avalanche) error {
	cli, err := NewGRPCClient(
		WithAvoidRPCVersionCheck(true),
		WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}
	defer cli.Close()
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	_, err = cli.Stop(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			app.Log.Debug("No local network running")
		} else {
			app.Log.Debug("failed stopping local network", zap.Error(err))
		}
	}

	pid, err := GetServerPID(app)
	if err != nil {
		return fmt.Errorf("failed getting PID from run file: %w", err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("could not find process with pid %d: %w", pid, err)
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("failed killing process with pid %d: %w", pid, err)
	}

	serverRunFilePath := app.GetRunFile()
	if err := os.Remove(serverRunFilePath); err != nil {
		return fmt.Errorf("failed removing run file %s: %w", serverRunFilePath, err)
	}
	return nil
}

func WatchServerProcess(serverCancel context.CancelFunc, errc chan error, log logging.Logger) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-sigc:
		log.Warn("signal received: %s; closing server", zap.String("signal", sig.String()))
		serverCancel()
		err := <-errc
		log.Warn("closed server: %s", zap.Error(err))
	case err := <-errc:
		log.Warn("server closed: %s", zap.Error(err))
		serverCancel()
	}
}
