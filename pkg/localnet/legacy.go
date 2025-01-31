// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanche-network-runner/server"
)

// Returns the ANR network info for the server endpoint
func GetANRNetworkInfoWithEndpoint(grpcServerEndpoint string) (*rpcpb.ClusterInfo, error) {
	cli, err := binutils.NewGRPCClientWithEndpoint(
		grpcServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return nil, err
	}
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	resp, err := cli.Status(ctx)
	if err != nil {
		return nil, err
	}
	return resp.GetClusterInfo(), nil
}

// Indicates is the client is connected to a bootstrapped network
// assumes server is up
func IsANRNetworkBootstrapped(ctx context.Context, cli client.Client) (bool, error) {
	_, err := cli.Status(ctx)
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			return false, nil
		}
		return false, fmt.Errorf("failed trying to get network status: %w", err)
	}
	return true, nil
}
