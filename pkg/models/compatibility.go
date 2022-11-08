package models

type VMCompatibility struct {
	RPCChainVMProtocolVersion map[string]int `json:"rpcChainVMProtocolVersion"`
}

type AvagoCompatiblity map[string][]string
