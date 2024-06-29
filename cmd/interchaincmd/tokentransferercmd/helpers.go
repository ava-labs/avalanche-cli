// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package tokentransferercmd

import (
	_ "embed"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
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

func promptChain(
	prompt string,
	network models.Network,
	avoidCChain bool,
	avoidSubnet string,
	chainFlags *ChainFlags,
) (bool, error) {
	subnetNames, err := app.GetSubnetNamesOnNetwork(network)
	if err != nil {
		return false, err
	}
	cancel, _, _, cChain, subnetName, err := prompts.PromptChain(
		app.Prompt,
		prompt,
		subnetNames,
		true,
		true,
		avoidCChain,
		avoidSubnet,
	)
	if err == nil {
		chainFlags.SubnetName = subnetName
		chainFlags.CChain = cChain
	}
	return cancel, err
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
