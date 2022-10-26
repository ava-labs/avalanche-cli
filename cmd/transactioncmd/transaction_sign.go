// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

const inputTxPathFlag = "input-tx-filepath"

var (
	inputTxPath string
	useLedger   bool
	keyName     string

	errNoSubnetID = errors.New("failed to find the subnet ID for this subnet, has it been deployed/created on this network?")
)

// avalanche transaction sign
func newTransactionSignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sign [subnetName]",
		Short:        "sign a transaction",
		Long:         "",
		RunE:         signTx,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&inputTxPath, inputTxPathFlag, "", "Path to the transaction file for signing")
	cmd.Flags().BoolVarP(&useLedger, "ledger", "g", false, "use ledger instead of key (always true on mainnet, defaults to false on fuji)")
	cmd.Flags().StringVarP(&keyName, "key", "k", "", "select the key to use [fuji only]")
	cmd.MarkFlagRequired(inputTxPathFlag)
	return cmd
}

func signTx(cmd *cobra.Command, args []string) error {
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

	// we need network to decide if ledger is required (mainnet)
	network, err := txutils.GetNetwork(tx)
	if err != nil {
		return err
	}
	switch network {
	case models.Fuji, models.Local:
		if !useLedger && keyName == "" {
			useLedger, keyName, err = prompts.GetFujiKeyOrLedger(app.Prompt, app.GetKeyDir())
			if err != nil {
				return err
			}
		}
	case models.Mainnet:
		useLedger = true
	default:
		return errors.New("unsupported network")
	}

	subnetName := args[0]
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[network.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, network, subnetID)
	if err != nil {
		return err
	}

	// get keychain accesor
	kc, err := key.GetKeychain(useLedger, app.GetKeyPath(keyName), network)
	if err != nil {
		return err
	}
	fmt.Println(remainingSubnetAuthKeys)
	fmt.Println(kc.Addresses())

	return nil
}
