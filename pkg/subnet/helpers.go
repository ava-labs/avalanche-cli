// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm"
)

func GetOwners(network models.Network, subnetID ids.ID) ([]string, uint32, error) {
	var api string
	switch network {
	case models.Fuji:
		api = constants.FujiAPIEndpoint
	case models.Mainnet:
		api = constants.MainnetAPIEndpoint
	case models.Local:
		api = constants.LocalAPIEndpoint
	default:
		return nil, 0, fmt.Errorf("network not supported")
	}
	pClient := platformvm.NewClient(api)
	ctx := context.Background()
	subnets, err := pClient.GetSubnets(ctx, []ids.ID{subnetID})
	if err != nil {
		return nil, 0, fmt.Errorf("subnet query error: %w", err)
	}
	if len(subnets) == 0 {
		return nil, 0, fmt.Errorf("subnet not found")
	}
	controlKeys := subnets[0].ControlKeys
	threshold := subnets[0].Threshold
	networkID, err := network.NetworkID()
	if err != nil {
		return nil, 0, err
	}
	hrp := key.GetHRP(networkID)
	controlKeysStrs := []string{}
	for _, addr := range controlKeys {
		addrStr, err := address.Format("P", hrp, addr[:])
		if err != nil {
			return nil, 0, err
		}
		controlKeysStrs = append(controlKeysStrs, addrStr)
	}
	return controlKeysStrs, threshold, nil
}
