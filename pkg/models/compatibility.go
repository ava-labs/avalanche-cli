// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package models

type VMCompatibility struct {
	RPCChainVMProtocolVersion map[string]int `json:"rpcChainVMProtocolVersion"`
}

type AvagoCompatiblity map[string][]string

type NetworkVersion struct {
	LatestVersion     string `json:"latest-version"`
	RequirePrerelease bool   `json:"require-prerelease"`
	PrereleaseVersion string `json:"prerelease-version"`
}

type CLIDependencyMap struct {
	SubnetEVM   string                    `json:"subnet-evm"`
	AvalancheGo map[string]NetworkVersion `json:"avalanchego"`
}
