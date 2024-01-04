// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import "os"

// IsE2E checks if the environment variable "RUN_E2E" is set and returns true if it is, false otherwise.
func IsE2E() bool {
	return os.Getenv("RUN_E2E") != ""
}
