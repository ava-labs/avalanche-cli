// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

type ChainFlags struct {
	SubnetName string
	CChain     bool
}

func AddChainFlagsToCmd(
	cmd *cobra.Command,
	chainFlags *ChainFlags,
	goal string,
	subnetFlagName string,
	cChainFlagName string,
) {
	if subnetFlagName == "" {
		subnetFlagName = "subnet"
	}
	if cChainFlagName == "" {
		cChainFlagName = "c-chain"
	}
	cmd.Flags().StringVar(&chainFlags.SubnetName, subnetFlagName, "", fmt.Sprintf("%s into the given CLI subnet", goal))
	cmd.Flags().BoolVar(&chainFlags.CChain, cChainFlagName, false, fmt.Sprintf("%s into C-Chain", goal))
}

func GetRPCURL(
	app *application.Avalanche,
	network models.Network,
	subnetName string,
	isCChain bool,
) (string, error) {
	if isCChain {
		return network.CChainEndpoint(), nil
	} else {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return "", fmt.Errorf("failed to load sidecar: %w", err)
		}
		if sc.Networks[network.Name()].BlockchainID == ids.Empty {
			return "", fmt.Errorf("subnet has not been deployed to %s", network.Name())
		}
		return network.BlockchainEndpoint(sc.Networks[network.Name()].BlockchainID.String()), nil
	}
}
