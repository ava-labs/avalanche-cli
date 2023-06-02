// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
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

	errNoSubnetID = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created on this network?")
)

// avalanche transaction sign
func newTransactionSignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sign [subnetName]",
		Short:        "sign a transaction",
		Long:         "The transaction sign command signs a multisig transaction.",
		RunE:         signTx,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
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

	if len(ledgerAddresses) > 0 {
		useLedger = true
	}

	if useLedger && keyName != "" {
		return subnetcmd.ErrMutuallyExlusiveKeyLedger
	}

	// we need network to decide if ledger is forced (mainnet)
	network, err := txutils.GetNetwork(tx)
	if err != nil {
		return err
	}
	switch network {
	case models.Fuji, models.Local:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, "sign transaction", app.GetKeyDir())
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
		if keyName != "" {
			return subnetcmd.ErrStoredKeyOnMainnet
		}
	default:
		return errors.New("unsupported network")
	}

	// we need subnet wallet signing validation + process
	subnetName := args[0]
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[network.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	controlKeys, _, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}

	// get the remaining tx signers so as to check that the wallet does contain an expected signer
	subnetAuthKeys, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return err
	}

	if len(remainingSubnetAuthKeys) == 0 {
		subnetcmd.PrintReadyToSignMsg(subnetName, inputTxPath)
		return nil
	}

	// get keychain accessor
	kc, err := subnetcmd.GetKeychain(useLedger, ledgerAddresses, keyName, network)
	if err != nil {
		return err
	}

	deployer := subnet.NewPublicDeployer(app, useLedger, kc, network)
	if err := deployer.Sign(tx, remainingSubnetAuthKeys, subnetID); err != nil {
		if errors.Is(err, subnet.ErrNoSubnetAuthKeysInWallet) {
			ux.Logger.PrintToUser("There are no required subnet auth keys present in the wallet")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("Expected one of:")
			for _, addr := range remainingSubnetAuthKeys {
				ux.Logger.PrintToUser("  %s", addr)
			}
			return nil
		}
		return err
	}

	// update the remaining tx signers after the signature has been done
	_, remainingSubnetAuthKeys, err = txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return err
	}

	if err := subnetcmd.SaveNotFullySignedTx(
		"Tx",
		tx,
		subnetName,
		subnetAuthKeys,
		remainingSubnetAuthKeys,
		inputTxPath,
		true,
	); err != nil {
		return err
	}

	return nil
}
