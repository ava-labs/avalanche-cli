// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
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

// Gathers blockchain info for all non standard blockchains at [endpoint]
func GetBlockchainsInfo(endpoint string) ([]BlockchainInfo, error) {
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

// Gathers blockchain info for all blockchains of [network] managed by CLI
func GetManagedBlockchainsInfo(app *application.Avalanche, network models.Network) ([]BlockchainInfo, error) {
	managedBlockchains, err := app.GetBlockchainNames()
	if err != nil {
		return nil, err
	}
	blockchainsInfo := []BlockchainInfo{}
	for _, managedBlockchain := range managedBlockchains {
		sc, err := app.LoadSidecar(managedBlockchain)
		if err != nil {
			return nil, err
		}
		var vmid ids.ID
		if sc.ImportedVMID != "" {
			vmid, err = ids.FromString(sc.ImportedVMID)
			if err != nil {
				return nil, err
			}
		} else {
			vmid, err = utils.VMID(sc.Name)
			if err != nil {
				return nil, err
			}
		}
		for networkName, networkInfo := range sc.Networks {
			if networkName == network.Name() {
				blockchainInfo := BlockchainInfo{
					Name:     sc.Name,
					ID:       networkInfo.BlockchainID,
					SubnetID: networkInfo.SubnetID,
					VMID:     vmid,
				}
				blockchainsInfo = append(blockchainsInfo, blockchainInfo)
			}
		}
	}
	return blockchainsInfo, nil
}
