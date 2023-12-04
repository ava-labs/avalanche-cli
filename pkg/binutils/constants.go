// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import "time"

const (
	gRPCClientLogLevel = "error"
	gRPCServerPort     = ":8097"
	gRPCGatewayPort    = ":8098"
	gRPCServerEndpoint = "localhost" + gRPCServerPort
	gRPCDialTimeout    = 10 * time.Second

	avalanchegoBinPrefix = "avalanchego-"
	subnetEVMBinPrefix   = "subnet-evm-"
	maxCopy              = 2147483648 // 2 GB
)
