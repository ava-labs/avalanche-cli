// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"fmt"
	"sort"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"golang.org/x/exp/maps"

	"github.com/ethereum/go-ethereum/common"
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

	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, getBalanceSupportedNetworkOptions)
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
		getBalanceSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	chainSpec := contract.ChainSpec{
		BlockchainName: blockchainName,
	}

	rpcURL, _, err := contract.GetBlockchainEndpoints(
		app,
		network,
		chainSpec,
		true,
		false,
	)
	if err != nil {
		return err
	}

	subnetID, err := contract.GetSubnetID(app, network, chainSpec)
	if err != nil {
		return err
	}

	pClient := platformvm.NewClient(network.Endpoint)
	ctx, cancel := utils.GetAPIContext()
	defer cancel()
	validators, err := pClient.GetValidatorsAt(ctx, subnetID, api.ProposedHeight)
	if err != nil {
		return err
	}

	managerAddress := common.HexToAddress(validatorManagerSDK.ProxyContractAddress)

	t := ux.DefaultTable(
		fmt.Sprintf("%s Validators", blockchainName),
		table.Row{"Node ID", "Validation ID", "Weight", "Remaining Balance (AVAX)"},
	)

	nodeIDs := maps.Keys(validators)
	nodeIDStrs := utils.Map(nodeIDs, func(nodeID ids.NodeID) string { return nodeID.String() })
	sort.Strings(nodeIDStrs)

	for _, nodeIDStr := range nodeIDStrs {
		nodeID, err := ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return err
		}
		validator := validators[nodeID]
		balance := uint64(0)
		validationID, err := validatorManagerSDK.GetRegisteredValidator(rpcURL, managerAddress, nodeID)
		if err != nil {
			ux.Logger.RedXToUser("could not get validation ID for node %s due to %s", nodeID, err)
		} else {
			balance, err = validatorManagerSDK.GetValidatorBalance(network.SDKNetwork(), validationID)
			if err != nil {
				ux.Logger.RedXToUser("could not get balance for node %s due to %s", nodeID, err)
			}
		}
		t.AppendRow(table.Row{nodeID, validationID, validator.Weight, float64(balance) / float64(units.Avax)})
	}
	fmt.Println(t.Render())
	return nil
}
