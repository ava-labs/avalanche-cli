// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists the validators of an L1",
		Long:  `This command gets a list of the validatos of the L1`,
		RunE:  list,
		Args:  cobrautils.ExactArgs(1),
	}

	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, getBalanceSupportedNetworkOptions)
	return cmd
}

func list(_ *cobra.Command, args []string) error {
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

	blockchainName := args[0]
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
		table.Row{"Node ID", "Validation ID", "Weight", "Remaining Balance"},
	)

	for nodeID, validator := range validators {
		validationID, err := validatormanager.GetRegisteredValidator(rpcURL, managerAddress, nodeID)
		if err != nil {
			return err
		}
		balance, err := txutils.GetValidatorPChainBalanceValidationID(network, validationID)
		if err != nil {
			return err
		}
		t.AppendRow(table.Row{nodeID, validationID, validator.Weight, float64(balance) / float64(units.Avax)})
	}
	fmt.Println(t.Render())
	return nil
}
