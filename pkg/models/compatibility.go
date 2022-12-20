// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package models

type VMCompatibility struct {
	RPCChainVMProtocolVersion map[string]int `json:"rpcChainVMProtocolVersion"`
}

type AvagoCompatiblity map[string][]string
