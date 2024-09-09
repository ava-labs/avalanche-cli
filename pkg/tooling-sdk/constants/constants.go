// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import "time"

const (
	// http
	APIRequestTimeout      = 30 * time.Second
	APIRequestLargeTimeout = 2 * time.Minute

	RemoteHostUser = "ubuntu"

	// node
	CloudNodeCLIConfigBasePath = "/home/ubuntu/.avalanche-cli/"
	WriteReadUserOnlyPerms     = 0o600
)
