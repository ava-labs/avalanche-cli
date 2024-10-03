// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import (
	_ "embed"
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
		"NodeID-P5QGH4EXddrcyNAzkqyZKHXgEpVX6HExL",
		"NodeID-78ibWpjtZz5ZGT6EyTEdu8VKmboUHTuGT",
		"NodeID-7eRvnfs2a2PvrPHUuCRRpPVAoVjbWxaFG",
		"NodeID-gpXWBExQSZXqJPQt6L6MnveUfgr7HJ4q",
		"NodeID-L4CY8B5uVSDe4cnN1BpeDsHacMp4q4q8q",
	}
	EtnaDevnetBootstrapIPs = []string{
		"107.21.11.213:9651",
		"34.233.248.130:9651",
		"52.201.126.172:9651",
		"35.170.144.5:9651",
		"98.82.41.186:9651",
		"34.228.34.127:9651",
		"44.205.136.166:9651",
		"52.6.31.40:9651",
		"54.197.98.148:9651",
		"18.211.108.228:9651",
	}
)
