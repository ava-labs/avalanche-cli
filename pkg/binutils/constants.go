// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import "time"

const (
	gRPCClientLogLevel = "error"
	gRPCDialTimeout    = 10 * time.Second

	avalanchegoBinPrefix = "avalanchego-"
	subnetEVMBinPrefix   = "subnet-evm-"
	maxCopy              = 2147483648 // 2 GB

	LocalNetworkGRPCServerPort     = ":8097"
	LocalNetworkGRPCGatewayPort    = ":8098"
	LocalNetworkGRPCServerEndpoint = "localhost" + LocalNetworkGRPCServerPort

	LocalClusterGRPCServerPort     = ":8090"
	LocalClusterGRPCGatewayPort    = ":8091"
	LocalClusterGRPCServerEndpoint = "localhost" + LocalClusterGRPCServerPort
)
