// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/validator"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [blockchainName]",
		Short: "Lists the validators of an L1",
		Long:  `This command gets a list of the validators of the L1`,
		RunE:  list,
		Args:  cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, networkoptions.DefaultSupportedNetworkOptions)
	return cmd
}

func list(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to load sidecar: %w", err)
	}
	if !sc.Sovereign {
		return fmt.Errorf("avalanche validator commands are only applicable to sovereign L1s")
	}

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		networkoptions.DefaultSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}

	subnetID, err := contract.GetSubnetID(app, network, chainSpec)
	if err != nil {
		return err
	}

	validators, err := validator.GetCurrentValidators(network.SDKNetwork(), subnetID)
	if err != nil {
		return err
	}

	t := ux.DefaultTable(
		fmt.Sprintf("%s Validators", blockchainName),
		table.Row{"Node ID", "Validation ID", "Weight", "Remaining Balance (AVAX)"},
	)
	for _, validator := range validators {
		t.AppendRow(table.Row{
			validator.NodeID,
			validator.ValidationID,
			validator.Weight,
			float64(validator.Balance) / float64(units.Avax),
		})
	}
	fmt.Println(t.Render())

	return nil
}
