// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm"
)

type BlockchainInfo struct {
	Name     string
	ID       ids.ID
	SubnetID ids.ID
	VMID     ids.ID
}

// Gathers blockchain info for all non standard blockchains
func GetBlockchainInfo(endpoint string) ([]BlockchainInfo, error) {
	pClient := platformvm.NewClient(endpoint)
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	blockchains, err := pClient.GetBlockchains(ctx)
	if err != nil {
		return nil, err
	}
	blockchainsInfo := []BlockchainInfo{}
	for _, blockchain := range blockchains {
		if blockchain.Name == "C-Chain" || blockchain.Name == "X-Chain" {
			continue
		}
		blockchainInfo := BlockchainInfo{
			Name:     blockchain.Name,
			ID:       blockchain.ID,
			SubnetID: blockchain.SubnetID,
			VMID:     blockchain.VMID,
		}
		blockchainsInfo = append(blockchainsInfo, blockchainInfo)
	}
	return blockchainsInfo, nil
}
