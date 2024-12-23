// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"fmt"
	"sort"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ethereum/go-ethereum/common"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

var globalNetworkFlags networkoptions.NetworkFlags

var (
	l1              string
	validationIDStr string
	nodeIDStr       string
)

var getBalanceSupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

func NewGetBalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "getBalance",
		Short: "Gets current balance of validator on P-Chain",
		Long: `This command gets the remaining validator P-Chain balance that is available to pay
P-Chain continuous fee`,
		RunE: getBalance,
		Args: cobrautils.ExactArgs(0),
	}

	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, getBalanceSupportedNetworkOptions)
	cmd.Flags().StringVar(&l1, "l1", "", "name of L1")
	cmd.Flags().StringVar(&validationIDStr, "validation-id", "", "validation ID of the validator")
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node ID of the validator")
	return cmd
}

func getBalance(_ *cobra.Command, _ []string) error {
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

	validationID, cancel, err := getNodeValidationID(network, l1, nodeIDStr, validationIDStr)
	if err != nil {
		return err
	}
	if cancel {
		return nil
	}
	if validationID == ids.Empty {
		return fmt.Errorf("the specified node is not a L1 validator")
	}

	balance, err := txutils.GetValidatorPChainBalanceValidationID(network, validationID)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("  Validator Balance: %.5f AVAX", float64(balance)/float64(units.Avax))

	return nil
}

// getNodeValidationID returns the node validation ID based on input
// present in given args and prompted to the user
// it also returns a boolean that is true if the user cancels
// operations during prompting
func getNodeValidationID(
	network models.Network,
	l1 string,
	nodeIDStr string,
	validationIDStr string,
) (ids.ID, bool, error) {
	var (
		validationID ids.ID
		err          error
	)
	if validationIDStr != "" {
		validationID, err = ids.FromString(validationIDStr)
		if err != nil {
			return validationID, false, err
		}
		return validationID, false, nil
	}
	l1ListOption := "I will choose from the L1 validators list"
	validationIDOption := "I know the validation ID"
	cancelOption := "Cancel"
	option := l1ListOption
	if l1 == "" && nodeIDStr == "" {
		options := []string{l1ListOption, validationIDOption, cancelOption}
		option, err = app.Prompt.CaptureList(
			"How do you want to specify the L1 validator",
			options,
		)
		if err != nil {
			return ids.Empty, false, err
		}
	}
	switch option {
	case l1ListOption:
		chainSpec := contract.ChainSpec{
			BlockchainName: l1,
		}
		chainSpec.SetEnabled(
			true,  // prompt blockchain name
			false, // do not prompt for PChain
			false, // do not prompt for XChain
			false, // do not prompt for CChain
			true,  // prompt blockchain ID
		)
		chainSpec.OnlySOV = true
		if l1 == "" {
			if cancel, err := contract.PromptChain(
				app,
				network,
				"Choose the L1",
				"",
				&chainSpec,
			); err != nil {
				return ids.Empty, false, err
			} else if cancel {
				return ids.Empty, true, nil
			}
			l1 = chainSpec.BlockchainName
		}
		if nodeIDStr == "" {
			if l1 != "" {
				sc, err := app.LoadSidecar(l1)
				if err != nil {
					return ids.Empty, false, fmt.Errorf("failed to load sidecar: %w", err)
				}
				if !sc.Sovereign {
					return ids.Empty, false, fmt.Errorf("avalanche validator commands are only applicable to sovereign L1s")
				}
			}
			subnetID, err := contract.GetSubnetID(app, network, chainSpec)
			if err != nil {
				return ids.Empty, false, err
			}
			pClient := platformvm.NewClient(network.Endpoint)
			ctx, cancel := utils.GetAPIContext()
			defer cancel()
			validators, err := pClient.GetValidatorsAt(ctx, subnetID, api.ProposedHeight)
			if err != nil {
				return ids.Empty, false, err
			}
			if len(validators) == 0 {
				return ids.Empty, false, fmt.Errorf("l1 has no validators")
			}
			nodeIDs := maps.Keys(validators)
			nodeIDStrs := utils.Map(nodeIDs, func(nodeID ids.NodeID) string { return nodeID.String() })
			sort.Strings(nodeIDStrs)
			nodeIDStr, err = app.Prompt.CaptureListWithSize("Choose Node ID of the validator", nodeIDStrs, 8)
			if err != nil {
				return ids.Empty, false, err
			}
		}
		nodeID, err := ids.NodeIDFromString(nodeIDStr)
		if err != nil {
			return ids.Empty, false, err
		}
		rpcURL, _, err := contract.GetBlockchainEndpoints(
			app,
			network,
			chainSpec,
			true,
			false,
		)
		if err != nil {
			return ids.Empty, false, err
		}
		managerAddress := common.HexToAddress(validatorManagerSDK.ProxyContractAddress)
		validationID, err = validatormanager.GetRegisteredValidator(rpcURL, managerAddress, nodeID)
		if err != nil {
			return ids.Empty, false, err
		}
	case validationIDOption:
		validationID, err = app.Prompt.CaptureID("What is the validator's validationID?")
		if err != nil {
			return ids.Empty, false, err
		}
	case cancelOption:
		return ids.Empty, true, nil
	}
	return validationID, false, nil
}
