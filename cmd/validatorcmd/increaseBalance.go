// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatorcmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"

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
	cmd.Flags().StringVar(&l1, "l1", "", "name of L1 (to get balance of bootstrap validators only)")
	cmd.Flags().StringVar(&subnetID, "subnet-id", "", "subnetID of L1 that the node is validating")
	cmd.Flags().StringVar(&validationIDStr, "validation-id", "", "validationIDStr of the validator")
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

		return nil
	} else {
		validationID, err = app.Prompt.CaptureID("What is the validator's validationID?")
		if err != nil {
			return err
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

	_, err = deployer.IncreaseValidatorPChainBalance(validationID, balance)
	if err != nil {
		return err
	}
	deployer.CleanCacheWallet()
	balance, err = txutils.GetValidatorPChainBalanceValidationID(network, validationID)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("  Validator Balance: %.5f", float64(balance)/float64(units.Avax))

	return nil
}
