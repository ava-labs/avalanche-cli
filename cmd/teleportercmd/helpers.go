// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"strings"
)

func isCChain(subnetName string) bool {
	return strings.ToLower(subnetName) == "c-chain" || strings.ToLower(subnetName) == "cchain"
}
