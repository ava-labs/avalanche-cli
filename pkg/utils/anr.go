package utils

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/server"
)

var ErrNetworkNotRunning = errors.New("no local network running")

func GetNetworkStatus() (*rpcpb.StatusResponse, error) {
	cli, err := binutils.NewGRPCClient()
	if err != nil {
		return nil, err
	}

	ctx := binutils.GetAsyncContext()
	status, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			return nil, ErrNetworkNotRunning
		}
		return nil, err
	}
	return status, nil
}
