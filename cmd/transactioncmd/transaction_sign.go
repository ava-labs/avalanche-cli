// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/spf13/cobra"
)

const inputTxPathFlag = "input-tx-filepath"

var (
	inputTxPath     string
	keyName         string
	useLedger       bool
	ledgerAddresses []string
)

// avalanche transaction sign
func newTransactionSignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign [blockchainName]",
		Short: "sign a transaction",
		Long:  "The transaction sign command signs a multisig transaction.",
		RunE:  signTx,
		Args:  cobrautils.MaximumNArgs(1),
	}

	cmd.Flags().StringVar(&inputTxPath, inputTxPathFlag, "", "Path to the transaction file for signing")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji only]")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringSliceVar(&ledgerAddresses, "ledger-addrs", []string{}, "use the given ledger addresses")
	return cmd
}

func signTx(_ *cobra.Command, args []string) error {
	var err error
	if inputTxPath == "" {
		inputTxPath, err = app.Prompt.CaptureExistingFilepath("What is the path to the transactions file which needs signing?")
		if err != nil {
			return err
		}
	}
	tx, err := txutils.LoadFromDisk(inputTxPath)
	if err != nil {
		return err
	}

	isConvertToL1Tx := txutils.IsConvertToL1Tx(tx)
	if isConvertToL1Tx {
		doContinue, err := validateConvertOperation(tx, "sign")
		if err != nil {
			return err
		}
		if !doContinue {
			return nil
		}
	}

	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return blockchaincmd.ErrMutuallyExlusiveKeyLedger
	}

	// we need network to decide if ledger is forced (mainnet)
	network, err := txutils.GetNetwork(tx)
	if err != nil {
		return err
	}
	switch network.Kind {
	case models.Local:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetKeyOrLedger(app.Prompt, "sign transaction", app.GetKeyDir(), true)
			if err != nil {
				return err
			}
		}
	case models.Fuji:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetKeyOrLedger(app.Prompt, "sign transaction", app.GetKeyDir(), false)
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
		if keyName != "" {
			return blockchaincmd.ErrStoredKeyOnMainnet
		}
	default:
		return errors.New("unsupported network")
	}

	// we need subnet ID for the wallet signing validation + process
	subnetID, err := txutils.GetSubnetID(tx)
	if err != nil {
		return err
	}
	var blockchainName string
	if len(args) > 0 {
		blockchainName = args[0]
		// subnet ID from tx is always preferred
		if subnetID == ids.Empty {
			sc, err := app.LoadSidecar(blockchainName)
			if err != nil {
				return err
			}
			subnetID = sc.Networks[network.Name()].SubnetID
			if subnetID == ids.Empty {
				return constants.ErrNoSubnetID
			}
		}
	}

	_, controlKeys, _, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}

	// get the remaining tx signers so as to check that the wallet does contain an expected signer
	subnetAuthKeys, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return err
	}

	if len(remainingSubnetAuthKeys) == 0 {
		blockchaincmd.PrintReadyToSignMsg(blockchainName, inputTxPath)
		ux.Logger.PrintToUser("")
		return fmt.Errorf("tx is already fully signed")
	}

	// get keychain accessor
	kc, err := keychain.GetKeychain(app, false, useLedger, ledgerAddresses, keyName, network, 0)
	if err != nil {
		return err
	}

	// add control keys to the keychain whenever possible
	if err := kc.AddAddresses(controlKeys); err != nil {
		return err
	}

	deployer := subnet.NewPublicDeployer(app, kc, network)
	if err := deployer.Sign(
		tx,
		remainingSubnetAuthKeys,
		subnetID,
	); err != nil {
		if errors.Is(err, subnet.ErrNoSubnetAuthKeysInWallet) {
			ux.Logger.PrintToUser("There are no required subnet auth keys present in the wallet")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("Expected one of:")
			for _, addr := range remainingSubnetAuthKeys {
				ux.Logger.PrintToUser("  %s", addr)
			}
			ux.Logger.PrintToUser("")
			return fmt.Errorf("no remaining signer address present in wallet")
		}
		return err
	}

	// update the remaining tx signers after the signature has been done
	_, remainingSubnetAuthKeys, err = txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return err
	}

	if err := blockchaincmd.SaveNotFullySignedTx(
		"Tx",
		tx,
		blockchainName,
		subnetAuthKeys,
		remainingSubnetAuthKeys,
		inputTxPath,
		true,
	); err != nil {
		return err
	}

	return nil
}
