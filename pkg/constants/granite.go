// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import (
	_ "embed"
)

//go:embed graniteDevnet/genesis.json
var GraniteDevnetGenesisData []byte

//go:embed graniteDevnet/upgrade.json
var GraniteDevnetUpgradeData []byte

var (
	GraniteDevnetBootstrapNodeIDs = []string{
		"NodeID-DG4zG5SNeMof4Fh6u5Dmn7qhpQD5pqbw1",
		"NodeID-HDkaXpmjrENuBmqkggonVQh6tmuFuJFP",
		"NodeID-PnfqkMjddeFVShZq8dwxb9KxsHgkES7RC",
		"NodeID-5T26eUHiPv276fQuM4y8dDsJrVJkHFm3L",
		"NodeID-M7StQdVYYzqWGHVZJsZoWjLWiWugB79DX",
	}
	GraniteDevnetBootstrapIPs = []string{
		"107.21.11.213:9651",
		"34.233.248.130:9651",
		"52.201.126.172:9651",
		"35.170.144.5:9651",
		"98.82.41.186:9651",
	}
)
