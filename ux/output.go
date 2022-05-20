// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ux

import (
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"
)

// PrintToUser prints msg directly on the screen, but also to log file
func PrintToUser(msg string, log logging.Logger) {
	fmt.Println(msg)
	log.Info(msg)
}
