// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/models"

	"github.com/spf13/cobra"
)

// avalanche subnet teleporter
func newTeleporterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "teleporter",
		Short:             "Deploys teleporter to local network cchain",
		Long:              `Deploys teleporter to a local network cchain.`,
		SilenceUsage:      true,
		RunE:              deployTeleporter,
		PersistentPostRun: handlePostRun,
		Args:              cobra.ExactArgs(0),
	}
	return cmd
}

func deployTeleporter(cmd *cobra.Command, args []string) error {
	url := models.LocalNetwork.CChainEndpoint()
	client, _ := evm.GetClient(url)
	fmt.Println(evm.GetAddressBalance(client, "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"))
	fmt.Println(evm.GetAddressBalance(client, "0x92c8461512e55F08c03F287B065eB521d93D8525"))
	fmt.Println()
	fmt.Println(evm.FundAddress(client, "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027", "0x92c8461512e55F08c03F287B065eB521d93D8525", big.NewInt(222222222)))
	fmt.Println()
	fmt.Println(evm.GetAddressBalance(client, "0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"))
	fmt.Println(evm.GetAddressBalance(client, "0x92c8461512e55F08c03F287B065eB521d93D8525"))
	return nil
}
