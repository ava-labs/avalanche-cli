// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import "time"

const (
	DefaultPerms755 = 0o755

	BaseDirName = ".avalanche-cli"

	SubnetEVMReleaseVersion   = "v0.2.3"
	AvalancheGoReleaseVersion = "v1.7.13"

	LatestAvagoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"
	SubnetEVMReleaseURL   = "https://api.github.com/repos/ava-labs/subnet-evm/releases/latest"

	ServerRunFile      = "gRPCserver.run"
	AvalancheCliBinDir = "bin"
	RunDir             = "runs"
	SidecarSuffix      = "_sidecar.json"
	GenesisSuffix      = "_genesis.json"

	SidecarVersion = "1.0.0"

	MaxLogFileSize   = 4
	MaxNumOfLogFiles = 5
	RetainOldFiles   = 0 // retain all old log files

	RequestTimeout = 3 * time.Minute

	DefaultTokenName = "TEST"

	HealthCheckInterval = 100 * time.Millisecond

	// it's unlikely anyone would want to name a snapshot `default`
	// but let's add some more entropy
	SnapshotsDirName             = "snapshots"
	DefaultSnapshotName          = "default-1654102509"
	BootstrapSnapshotURL         = "https://github.com/ava-labs/avalanche-cli/raw/main/assets/bootstrapSnapshot.tar.gz"
	BootstrapSnapshotArchiveName = "bootstrapSnapshot.tar.gz"
)
