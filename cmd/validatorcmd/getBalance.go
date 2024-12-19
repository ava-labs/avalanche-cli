// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/spf13/cobra"
)

var globalNetworkFlags networkoptions.NetworkFlags

var (
	l1              string
	validationIDStr string
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
	cmd.Flags().StringVar(&l1, "l1", "", "name of L1 (required to get balance of bootstrap validators)")
	cmd.Flags().StringVar(&validationIDStr, "validation-id", "", "validationIDStr of the validator")
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

	var balance uint64
	if validationIDStr != "" {
		validationID, err := ids.FromString(validationIDStr)
		if err != nil {
			return err
		}
		balance, err = txutils.GetValidatorPChainBalanceValidationID(network, validationID)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("  Validator Balance: %.5f", float64(balance)/float64(units.Avax))
		return nil
	}

	isBootstrapValidator, err := app.Prompt.CaptureYesNo("Is the validator a bootstrap validator?")
	if err != nil {
		return err
	}
	if isBootstrapValidator {
		if l1 == "" {
			return fmt.Errorf("--l1 flag is required to get bootstrap validator balance")
		}
		sc, err := app.LoadSidecar(l1)
		if err != nil {
			return fmt.Errorf("failed to load sidecar: %w", err)
		}
		if !sc.Sovereign {
			return fmt.Errorf("avalanche validator getBalance command is only applicable to sovereign L1s")
		}
		bootstrapValidators := sc.Networks[network.Name()].BootstrapValidators
		if len(bootstrapValidators) == 0 {
			return fmt.Errorf("this L1 does not have any bootstrap validators")
		}
		bootstrapValidatorsString := []string{}
		bootstrapValidatorsToIndexMap := make(map[string]int)
		for index, validator := range bootstrapValidators {
			bootstrapValidatorsString = append(bootstrapValidatorsString, validator.NodeID)
			bootstrapValidatorsToIndexMap[validator.NodeID] = index
		}
		chosenValidator, err := app.Prompt.CaptureList("Which bootstrap validator do you want to get balance of?", bootstrapValidatorsString)
		if err != nil {
			return err
		}
		validationID, err := ids.FromString(bootstrapValidators[bootstrapValidatorsToIndexMap[chosenValidator]].ValidationID)
		if err != nil {
			return err
		}
		balance, err = txutils.GetValidatorPChainBalanceValidationID(network, validationID)
		if err != nil {
			return err
		}
	} else {
		validationID, err := app.Prompt.CaptureID("What is the validator's validationID?")
		if err != nil {
			return err
		}
		balance, err = txutils.GetValidatorPChainBalanceValidationID(network, validationID)
		if err != nil {
			return err
		}
	}
	ux.Logger.PrintToUser("  Validator Balance: %.5f AVAX", float64(balance)/float64(units.Avax))

	return nil
}
