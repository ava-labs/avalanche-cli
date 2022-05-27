// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import "time"

const (
	DefaultPerms755 = 0o755

	LatestAvagoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"
	ServerRunFile         = "/tmp/gRPCserver.run"
	AvalancheCliBinDir    = "bin"

	RequestTimeout = 3 * time.Minute
)
