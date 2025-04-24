// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import "time"

const (
	// http
	APIRequestTimeout      = 10 * time.Second
	APIRequestLargeTimeout = 30 * time.Second

	// node
	UserOnlyWriteReadPerms     = 0o600
	UserOnlyWriteReadExecPerms = 0o700
)
