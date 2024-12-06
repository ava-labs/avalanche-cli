// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package constants

import (
	_ "embed"
)

//go:embed localNetwork/genesis.json
var LocalNetworkGenesisData []byte

//go:embed localNetwork/upgrade.json
var LocalNetworkUpgradeData []byte

var (
	LocalNetworkBootstrapNodeIDs = []string{
		"NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg",
		"NodeID-MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ",
	}
	LocalNetworkBootstrapIPs = []string{
		"127.0.0.1:9650",
		"127.0.0.1:9652",
	}
)
