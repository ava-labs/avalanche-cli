// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import (
	_ "embed"
	"time"

	"github.com/ava-labs/avalanchego/upgrade"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
)

//go:embed etnaDevnet/genesis.json
var EtnaDevnetGenesisData []byte

//go:embed etnaDevnet/upgrade.json
var EtnaDevnetUpgradeData []byte

const (
	EtnaDevnetEndpoint  = "https://etna.avax-dev.network"
	EtnaDevnetNetworkID = uint32(76)
)

var (
	EtnaDevnetBootstrapNodeIDs = []string{
		"NodeID-WrLWMK5sJ4dBUAsx1dP2FUyTqrYwbFA1",
		"NodeID-bojBKDrpt81bYhxYKQfLw89V7CpoH2m7",
		"NodeID-8LbTmmGsDC991SbD8Nkx88VULT3XYzYXC",
		"NodeID-DDhXtFm6Q9tCq2yiFRmcSMKvHgUgh8yQC",
		"NodeID-QDYnWDQd6g4cQ5H6yiWNqSmfRMBqEH9AG",
	}
	EtnaDevnetBootstrapIPs = []string{
		"107.21.11.213:9651",
		"34.233.248.130:9651",
		"52.201.126.172:9651",
		"35.170.144.5:9651",
		"98.82.41.186:9651",
	}
)

const StakingEtnaMinimumDuration = 100 * time.Second

var EtnaActivationTime = map[uint32]time.Time{
	avagoconstants.FujiID: time.Date(2024, time.November, 25, 16, 0, 0, 0, time.UTC),
	EtnaDevnetNetworkID:   time.Date(2024, time.October, 9, 20, 0, 0, 0, time.UTC),
	LocalNetworkID:        upgrade.Default.EtnaTime,
}
