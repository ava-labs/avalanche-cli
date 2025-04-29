// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	pb "github.com/ava-labs/avalanchego/proto/pb/vm/runtime"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/grpcutils"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/gruntime"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/runtime"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/runtime/subprocess"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
)

// protocolVersionQueryInitializer gets vm protocol version during handshake and provides it on a channel

var _ runtime.Initializer = (*protocolVersionQueryInitializer)(nil)

type protocolVersionQueryInitializer struct {
	protocolVersionCh chan uint
}

func newProtocolVersionQueryInitializer() *protocolVersionQueryInitializer {
	return &protocolVersionQueryInitializer{
		protocolVersionCh: make(chan uint),
	}
}

func (i *protocolVersionQueryInitializer) Initialize(_ context.Context, protocolVersion uint, _ string) error {
	i.protocolVersionCh <- protocolVersion
	return nil
}

func GetVMBinaryProtocolVersion(vmPath string) (int, error) {
	// get a network listener on a fresh local port
	listener, err := grpcutils.NewListener()
	if err != nil {
		return 0, fmt.Errorf("failed to create listener: %w", err)
	}
	defer listener.Close()

	// get a grpc server with default options. it is not accepting requests yet and has no services registered
	server := grpcutils.NewServer()
	defer server.GracefulStop()

	// an initializer abstracts protocol version checks during node/vm handshake
	// in this case we use the initializer to get the vm protocol version on a channel
	versionQueryInitializer := newProtocolVersionQueryInitializer()

	// get a runtime service to be used during vm handshake
	// a vm always calls the Initialize method of this service to notify the protocol version as part of the node/vm initialization handshake
	runtimeService := gruntime.NewServer(versionQueryInitializer)

	// register the runtime service to the grpc server
	pb.RegisterRuntimeServer(server, runtimeService)

	// start serving the runtime service
	go grpcutils.Serve(listener, server)

	// get absolute path of vm executable and create cmd
	absoluteVMPath, err := filepath.Abs(vmPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get absolute path for %s: %w", vmPath, err)
	}
	cmd := subprocess.NewCmd(absoluteVMPath)

	// configure EngineAddressKey vm environment variable so the vm knows where to locate the runtime service
	serverAddr := listener.Addr()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", runtime.EngineAddressKey, serverAddr.String()))

	// get plugin stdout/stderr plugins
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to get vm stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to get vm stderr pipe: %w", err)
	}

	// start the vm
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start vm: %w", err)
	}

	// define handshake timeout
	timeout := time.NewTimer(runtime.DefaultHandshakeTimeout)
	defer timeout.Stop()

	// wait for protocol version or timeout
	var protocolVersion uint
	select {
	case protocolVersion = <-versionQueryInitializer.protocolVersionCh:
	case <-timeout.C:
		_ = dumpProcessOutput(stdoutPipe, stderrPipe)
		return 0, fmt.Errorf("timeout while waiting for vm protocol version: %w", runtime.ErrHandshakeFailed)
	}

	// no need for a clean process termination
	if err := cmd.Process.Kill(); err != nil {
		_ = dumpProcessOutput(stdoutPipe, stderrPipe)
		return 0, fmt.Errorf("failure killing vm: %w", err)
	}

	return int(protocolVersion), nil
}

func dumpProcessOutput(stdoutPipe io.ReadCloser, stderrPipe io.ReadCloser) error {
	stdout, err := io.ReadAll(stdoutPipe)
	if err != nil {
		return err
	}
	stderr, err := io.ReadAll(stderrPipe)
	if err != nil {
		return err
	}
	fmt.Println(string(stdout))
	fmt.Println(string(stderr))
	return nil
}

func GetRPCProtocolVersion(app *application.Avalanche, vmType models.VMType, vmVersion string) (int, error) {
	var url string

	switch vmType {
	case models.SubnetEvm:
		url = constants.SubnetEVMRPCCompatibilityURL
	default:
		return 0, errors.New("unknown VM type")
	}

	compatibilityBytes, err := app.Downloader.Download(url)
	if err != nil {
		return 0, err
	}

	var parsedCompat models.VMCompatibility
	if err = json.Unmarshal(compatibilityBytes, &parsedCompat); err != nil {
		return 0, err
	}

	version, ok := parsedCompat.RPCChainVMProtocolVersion[vmVersion]
	if !ok {
		return 0, errors.New("no RPC version found")
	}

	return version, nil
}
