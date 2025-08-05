// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package models

type VMCompatibility struct {
	RPCChainVMProtocolVersion map[string]int `json:"rpcChainVMProtocolVersion"`
}

type AvagoCompatiblity map[string][]string

type NetworkVersion struct {
	LatestVersion  string `json:"latest-version"`
	MinimumVersion string `json:"minimum-version"`
}

type CLIDependencyMap struct {
	RPC                 int                       `json:"rpc"`
	SubnetEVM           string                    `json:"subnet-evm"`
	AvalancheGo         map[string]NetworkVersion `json:"avalanchego"`
	SignatureAggregator string                    `json:"signature-aggregator"`
}
