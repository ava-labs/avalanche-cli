// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/spf13/cobra"
)

var (
	keyName         string
	useLedger       bool
	useEwoq         bool
	ledgerAddresses []string
	balanceFlag     float64
)

var increaseBalanceSupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
	networkoptions.EtnaDevnet,
	networkoptions.Fuji,
	networkoptions.Mainnet,
}

func NewIncreaseBalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "increaseBalance",
		Short: "Increase current balance of validator on P-Chain",
		Long:  `This command increases the validator P-Chain balance`,
		RunE:  increaseBalance,
		Args:  cobrautils.ExactArgs(0),
	}

	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, increaseBalanceSupportedNetworkOptions)
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet deploy only]")
	cmd.Flags().StringVar(&l1, "l1", "", "name of L1 (to increase balance of bootstrap validators only)")
	cmd.Flags().StringVar(&validationIDStr, "validation-id", "", "validationIDStr of the validator")
	cmd.Flags().Float64Var(&balanceFlag, "balance", 0, "amount of AVAX to increase validator's balance by")
	return cmd
}

func increaseBalance(_ *cobra.Command, _ []string) error {
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
	var validationID ids.ID
	if validationIDStr != "" {
		validationID, err = ids.FromString(validationIDStr)
		if err != nil {
			return err
		}
	} else {
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
				return fmt.Errorf("avalanche validator increaseBalance command is only applicable to sovereign L1s")
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
			validationID, err = ids.FromString(bootstrapValidators[bootstrapValidatorsToIndexMap[chosenValidator]].ValidationID)
			if err != nil {
				return err
			}
		} else {
			validationID, err = app.Prompt.CaptureID("What is the validator's validationID?")
			if err != nil {
				return err
			}
		}
	}
	fee := network.GenesisParams().TxFeeConfig.StaticFeeConfig.TxFee
	kc, err := keychain.GetKeychainFromCmdLineFlags(
		app,
		constants.PayTxsFeesMsg,
		network,
		keyName,
		useEwoq,
		useLedger,
		ledgerAddresses,
		fee,
	)
	if err != nil {
		return err
	}
	deployer := subnet.NewPublicDeployer(app, kc, network)
	if balanceFlag == 0 {
		availableBalance, err := utils.GetNetworkBalance(kc.Addresses().List(), network.Endpoint)
		if err != nil {
			return err
		}
		balance, err = promptValidatorBalance(availableBalance / units.Avax)
		if err != nil {
			return err
		}
	} else {
		balance = uint64(balanceFlag * float64(units.Avax))
	}

	_, err = deployer.IncreaseValidatorPChainBalance(validationID, balance)
	if err != nil {
		return err
	}
	deployer.CleanCacheWallet()
	balance, err = txutils.GetValidatorPChainBalanceValidationID(network, validationID)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("  New Validator Balance: %.5f AVAX", float64(balance)/float64(units.Avax))

	return nil
}

func promptValidatorBalance(availableBalance uint64) (uint64, error) {
	ux.Logger.PrintToUser("Validator's balance is used to pay for continuous fee to the P-Chain")
	ux.Logger.PrintToUser("When this Balance reaches 0, the validator will be considered inactive and will no longer participate in validating the L1")
	txt := "How many AVAX do you want to increase the balance of this validator by?"
	return app.Prompt.CaptureValidatorBalance(txt, availableBalance, 0)
}
