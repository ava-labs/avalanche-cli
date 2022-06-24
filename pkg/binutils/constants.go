// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import "time"

const (
	gRPCClientLogLevel  = "error"
	gRPCServerEndpoint  = ":8097"
	gRPCGatewayEndpoint = ":8098"
	gRPCDialTimeout     = 10 * time.Second

	subnetEVMName = "subnet-evm"
	maxCopy       = 2147483648 // 2 GB
)
