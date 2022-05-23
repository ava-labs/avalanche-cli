// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import "time"

const (
	DefaultPerms755 = 0o755

	LatestAvagoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"
	// TODO we can not release with this...
	BinaryServerURL    = "http://3.84.91.164:8998"
	ServerRunFile      = "/tmp/gRPCserver.run"
	AvalancheCliBinDir = "bin"

	RequestTimeout = 3 * time.Minute
)
