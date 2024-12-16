// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import (
	_ "embed"
	"time"

	"github.com/ava-labs/avalanchego/upgrade"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
)

var EtnaActivationTime = map[uint32]time.Time{
	avagoconstants.FujiID: time.Date(2024, time.November, 25, 16, 0, 0, 0, time.UTC),
	LocalNetworkID:        upgrade.Default.EtnaTime,
}
