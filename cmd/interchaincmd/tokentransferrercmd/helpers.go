// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tokentransferrercmd

import (
	_ "embed"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
)

func validateSubnet(network models.Network, subnetName string) error {
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("subnet %s not deployed into %s", subnetName, network.Name())
	}
	return nil
}

func getNativeTokenSymbol(subnetName string, isCChain bool) (string, error) {
	nativeTokenSymbol := "AVAX"
	if !isCChain {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return "", err
		}
		nativeTokenSymbol = sc.TokenSymbol
	}
	return nativeTokenSymbol, nil
}
