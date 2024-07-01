// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"

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
