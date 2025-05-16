// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package configure

import (
	"fmt"
)

func getBlockchainConfig(rpcTxFeeCap int) string {
	return fmt.Sprintf("{\"rpc-tx-fee-cap\": %d}", rpcTxFeeCap)
}

func getPerNodeChainConfig(nodesRPCTxFeeCap map[string]int) string {
	perNodeChainConfig := "{\n"
	commaStr := ","
	i := 0
	for nodeID, rpcTxFeeCap := range nodesRPCTxFeeCap {
		if i == len(nodesRPCTxFeeCap)-1 {
			commaStr = ""
		}
		perNodeChainConfig += fmt.Sprintf("  \"%s\": {\"rpc-tx-fee-cap\": %d}%s\n", nodeID, rpcTxFeeCap, commaStr)
		i++
	}
	perNodeChainConfig += "}\n"
	return perNodeChainConfig
}

func subnetConfigLog(nodeID string) string {
	if nodeID == "" {
		return "\"validatorOnly\":false,\"allowedNodes\":[]"
	}
	return fmt.Sprintf("\"validatorOnly\":true,\"allowedNodes\":[\"%s\"]", nodeID)
}

func getSubnetConfig(nodeID string) string {
	return fmt.Sprintf("{\"validatorOnly\": true, \"allowedNodes\": [\"%s\"]}", nodeID)
}

func getNodeConfig(acpSupport int) string {
	return fmt.Sprintf("{\"acp-support\": %d}", acpSupport)
}
