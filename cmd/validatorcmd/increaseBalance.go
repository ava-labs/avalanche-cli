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
	balanceFlt      float64
)

var increaseBalanceSupportedNetworkOptions = []networkoptions.NetworkOption{
	networkoptions.Local,
	networkoptions.Devnet,
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
	cmd.Flags().StringVar(&nodeIDStr, "node-id", "", "node ID of the validator")
	cmd.Flags().Float64Var(&balanceFlt, "balance", 0, "amount of AVAX to increase validator's balance by")
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

	var balance uint64
	if balanceFlt == 0 {
		availableBalance, err := utils.GetNetworkBalance(kc.Addresses().List(), network.Endpoint)
		if err != nil {
			return err
		}
		balance, err = promptValidatorBalance(availableBalance / units.Avax)
		if err != nil {
			return err
		}
	} else {
		balance = uint64(balanceFlt * float64(units.Avax))
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
