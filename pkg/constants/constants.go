// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import "time"

const (
	DefaultPerms755 = 0o755

	LatestAvagoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"
	BinaryServerURL       = "http://3.84.91.164:8998"
	ServerRunFile         = "/tmp/gRPCserver.run"
	AvalancheCliBinDir    = "bin"

	GRPCClientLogLevel  = "error"
	GRPCServerEndpoint  = ":8097"
	GRPCGatewayEndpoint = ":8098"
	GRPCDialTimeout     = 10 * time.Second
)
