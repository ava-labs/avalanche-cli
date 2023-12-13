// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package models

import "github.com/ava-labs/avalanchego/version"

type VMCompatibility struct {
	RPCChainVMProtocolVersion map[string]int `json:"rpcChainVMProtocolVersion"`
}

type AvagoCompatiblity map[uint][]*version.Semantic
