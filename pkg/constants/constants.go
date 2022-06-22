// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import "time"

const (
	DefaultPerms755 = 0o755

	SubnetEVMReleaseVersion   = "v0.2.3"
	AvalancheGoReleaseVersion = "v1.7.13"

	LatestAvagoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"
	SubnetEVMReleaseURL   = "https://api.github.com/repos/ava-labs/subnet-evm/releases/latest"

	ServerRunFile      = "/tmp/gRPCserver.run"
	AvalancheCliBinDir = "bin"
	Sidecar_suffix     = "_sidecar.json"
	Genesis_suffix     = "_genesis.json"

	SidecarVersion = "1.0.0"

	RequestTimeout = 3 * time.Minute

	// it's unlikely anyone would want to name a snapshot `default`
	// but let's add some more entropy
	SnapshotsDirName     = "snapshots"
	DefaultSnapshotName  = "default-1654102509"
	BootstrapSnapshotURL = "https://github.com/ava-labs/avalanche-cli/raw/fast-deploy-with-default-snapshot-fix/assets/bootstrapSnapshot.tar.gz"
)
