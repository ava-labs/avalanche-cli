// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import "time"

const (
	BaseDirName    = ".avalanche-cli"
	sidecar_suffix = "_sidecar.json"
	genesis_suffix = "_genesis.json"

	subnetEvm = "SubnetEVM"
	customVm  = "Custom"

	latestAvagoReleaseURL = "https://api.github.com/repos/ava-labs/avalanchego/releases/latest"
	binaryServerURL       = "http://3.84.91.164:8998"
	serverRun             = "/tmp/gRPCserver.run"
	avalancheCliBinDir    = "bin"

	gRPCClientLogLevel = "error"
	gRPCServerEndpoint = "0.0.0.0:8097"
	gRPCDialTimeout    = 10 * time.Second
)
