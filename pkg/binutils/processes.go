// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
)

// ErrGRPCTimeout is a common error message if the gRPC server can't be reached
var ErrGRPCTimeout = errors.New("timed out trying to contact backend controller, it is most probably not running")

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
	return NewGRPCClientWithEndpoint(LocalNetworkGRPCServerEndpoint, opts...)
}

func NewGRPCClientWithEndpoint(
	serverEndpoint string,
	opts ...GRPCClientOpOption,
) (client.Client, error) {
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
		Endpoint:    serverEndpoint,
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

type runFile struct {
	Pid                int    `json:"pid"`
	GRPCserverFileName string `json:"gRPCserverFileName"`
}

func GetServerPID(
	app *application.Avalanche,
	prefix string,
) (int, error) {
	var rf runFile
	serverRunFilePath := app.GetRunFile(prefix)
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

func KillgRPCServerProcess(
	app *application.Avalanche,
	serverEndpoint string,
	prefix string,
) error {
	cli, err := NewGRPCClientWithEndpoint(
		serverEndpoint,
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

	pid, err := GetServerPID(app, prefix)
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

	serverRunFilePath := app.GetRunFile(prefix)
	if err := os.Remove(serverRunFilePath); err != nil {
		return fmt.Errorf("failed removing run file %s: %w", serverRunFilePath, err)
	}
	return nil
}
