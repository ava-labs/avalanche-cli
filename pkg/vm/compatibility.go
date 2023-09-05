// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	pb "github.com/ava-labs/avalanchego/proto/pb/vm/runtime"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/grpcutils"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/gruntime"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/runtime"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/runtime/subprocess"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"golang.org/x/mod/semver"
)

var ErrNoAvagoVersion = errors.New("unable to find a compatible avalanchego version")

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

	// configure EngineAddresKey vm environment variable so the vm knows where to locate the runtime service
	serverAddr := listener.Addr()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", runtime.EngineAddressKey, serverAddr.String()))

	// get plugin stdout/stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// start the vm
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	// define handshake timeout
	timeout := time.NewTimer(runtime.DefaultHandshakeTimeout)
	defer timeout.Stop()

	// wait for protocol version or timeout
	var protocolVersion uint
	select {
	case protocolVersion = <-versionQueryInitializer.protocolVersionCh:
	case <-timeout.C:
		return 0, fmt.Errorf("%w", runtime.ErrHandshakeFailed)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return 0, err
	}

	if err := cmd.Wait(); err != nil {
		return 0, err
	}

	return int(protocolVersion), nil
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

// GetAvalancheGoVersionsForRPC returns list of compatible avalanche go versions for a specified rpcVersion
func GetAvalancheGoVersionsForRPC(app *application.Avalanche, rpcVersion int, url string) ([]string, error) {
	compatibilityBytes, err := app.Downloader.Download(url)
	if err != nil {
		return nil, err
	}

	var parsedCompat models.AvagoCompatiblity
	if err = json.Unmarshal(compatibilityBytes, &parsedCompat); err != nil {
		return nil, err
	}

	eligibleVersions, ok := parsedCompat[strconv.Itoa(rpcVersion)]
	if !ok {
		return nil, ErrNoAvagoVersion
	}

	// versions are not necessarily sorted, so we need to sort them, tho this puts them in ascending order
	semver.Sort(eligibleVersions)
	return eligibleVersions, nil
}

// GetAvailableAvalancheGoVersions returns list of only available for download avalanche go versions,
// with latest version in first index
func GetAvailableAvalancheGoVersions(app *application.Avalanche, rpcVersion int, url string) ([]string, error) {
	eligibleVersions, err := GetAvalancheGoVersionsForRPC(app, rpcVersion, url)
	if err != nil {
		return nil, ErrNoAvagoVersion
	}
	// get latest avago release to make sure we're not picking a release currently in progress but not available for download
	latestAvagoVersion, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
	))
	if err != nil {
		return nil, err
	}
	var availableVersions []string
	for i := len(eligibleVersions) - 1; i >= 0; i-- {
		versionComparison := semver.Compare(eligibleVersions[i], latestAvagoVersion)
		if versionComparison != 1 {
			availableVersions = append(availableVersions, eligibleVersions[i])
		}
	}
	if len(availableVersions) == 0 {
		return nil, ErrNoAvagoVersion
	}
	return availableVersions, nil
}

func GetLatestAvalancheGoByProtocolVersion(app *application.Avalanche, rpcVersion int, url string) (string, error) {
	useVersion, err := GetAvailableAvalancheGoVersions(app, rpcVersion, url)
	if err != nil {
		return "", err
	}
	return useVersion[0], nil
}
