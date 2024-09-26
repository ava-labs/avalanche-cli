// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"errors"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/networkoptions"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
	"os"
)

var ()

// avalanche blockchain addValidator
func newChangeWeightCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "changeWeight [blockchainName] [nodeID]",
		Short: "Changes the weight of a Subnet validator",
		Long: `The blockchain changeWeight command changes the weight of a Subnet Validator.

The Subnet has to be a Proof of Authority Subnet-Only Validator Subnet.`,
		RunE: updateWeight,
		Args: cobrautils.ExactArgs(1),
	}
	networkoptions.AddNetworkFlagsToCmd(cmd, &globalNetworkFlags, true, addValidatorSupportedNetworkOptions)

	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji/devnet only]")
	cmd.Flags().StringVar(&nodeIDStr, "nodeID", "", "set the NodeID of the validator to add")
	cmd.Flags().Uint64Var(&weight, "weight", constants.BootstrapValidatorWeight, "set the staking weight of the validator to add")
	cmd.Flags().BoolVarP(&useEwoq, "ewoq", "e", false, "use ewoq key [fuji/devnet only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji/devnet)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	cmd.Flags().BoolVar(&nonSOV, "not-sov", false, "set to true if adding validator to a non SOV blockchain")
	cmd.Flags().StringVar(&publicKey, "public-key", "", "set the BLS public key of the validator to add")
	cmd.Flags().StringVar(&pop, "proof-of-possession", "", "set the BLS proof of possession of the validator to add")
	return cmd
}

func updateWeight(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	// TODO: validate node id format
	nodeID := args[1]
	var err error

	//TODO: add check for non SOV subnet
	// return err if non SOV

	// TODO: check for number of validators
	// return error if there is only 1 validator

	network, err := networkoptions.GetNetworkFromCmdLineFlags(
		app,
		"",
		globalNetworkFlags,
		true,
		false,
		removeValidatorSupportedNetworkOptions,
		"",
	)
	if err != nil {
		return err
	}

	if outputTxPath != "" {
		if _, err := os.Stat(outputTxPath); err == nil {
			return fmt.Errorf("outputTxPath %q already exists", outputTxPath)
		}
	}

	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return ErrMutuallyExlusiveKeyLedger
	}

	switch network.Kind {
	case models.Devnet:
		if useLedger {
			return ErrLedgerOnDevnet
		}
		keyName, err = prompts.CaptureKeyName(app.Prompt, constants.PayTxsFeesMsg, app.GetKeyDir(), false)
		if err != nil {
			return err
		}
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetKeyOrLedger(app.Prompt, constants.PayTxsFeesMsg, app.GetKeyDir(), false)
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
		if keyName != "" {
			return ErrStoredKeyOnMainnet
		}
	default:
		return errors.New("unsupported network")
	}

	// get keychain accesor
	fee := network.GenesisParams().TxFeeConfig.StaticFeeConfig.TxFee
	kc, err := keychain.GetKeychain(app, false, useLedger, ledgerAddresses, keyName, network, fee)
	if err != nil {
		return err
	}

	network.HandlePublicNetworkSimulation()

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	deployer := subnet.NewPublicDeployer(app, kc, network)

	// first remove the validator from subnet
	err = removeValidatorSOV(deployer)
	if err != nil {
		return err
	}

	// TODO: we need to wait for the balance from the removed validator to arrive in changeAddr
	// set arbitrary time.sleep here?

	weight, err = promptWeightSubnetValidator()
	if err != nil {
		return err
	}

	balance, err = getValidatorBalanceFromPChain()
	if err != nil {
		return err
	}

	publicKey, pop, err = getBLSInfoFromPChain()
	if err != nil {
		return err
	}

	changeAddr, err = getChangeAddrFromPChain()
	if err != nil {
		return fmt.Errorf("failure parsing change owner address: %w", err)
	}

	// add back validator to subnet with updated weight
	return CallAddValidator(deployer, network, kc, useLedger, blockchainName, nodeID)
}

func promptWeightSubnetValidator() (uint64, error) {
	txt := "What weight would you like to assign to the validator?"
	return app.Prompt.CaptureWeight(txt)
}

// getValidatorBalanceFromPChain gets remaining balance of validator from p chain
func getValidatorBalanceFromPChain() (uint64, error) {
	return 0, nil
}

// getBLSInfoFromPChain gets BLS public key and pop from info api
func getBLSInfoFromPChain() (string, string, error) {
	return "", "", nil
}

// getChangeAddrFromPChain gets validator change addr from info api
func getChangeAddrFromPChain() (string, error) {
	return "", nil
}
