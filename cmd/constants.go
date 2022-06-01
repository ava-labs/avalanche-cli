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

	// it's unlikely anyone would want to name a snapshot `default`
	// but let's add some more entropy
	defaultSnapshotName = "default-1654102509"
	healthCheckInterval = 10 * time.Second
)
