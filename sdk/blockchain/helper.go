// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockchain

import (
	"github.com/ava-labs/avalanchego/vms/platformvm"

	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanche-cli/sdk/utils"

	"github.com/ava-labs/avalanchego/ids"
)

func GetSubnet(subnetID ids.ID, network network.Network) (platformvm.GetSubnetClientResponse, error) {
	api := network.Endpoint
	pClient := platformvm.NewClient(api)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	return pClient.GetSubnet(ctx, subnetID)
}
