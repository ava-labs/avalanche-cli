package models

type VMCompatibility struct {
	RPCChainVMProtocolVersion map[string]int `json:"rpcChainVMProtocolVersion"`
}

type AvagoCompatiblity struct {
	RPCChainVMProtocolVersion map[string][]string `json:"rpcChainVMProtocolVersion"`
}
