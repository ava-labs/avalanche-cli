// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/spf13/cobra"
)

// avalanche transaction commit
func newTransactionCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "commit [subnetName]",
		Short:        "commit a transaction",
		Long:         "The transaction commit command commits a transaction by submitting it to the P-Chain.",
		RunE:         commitTx,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&inputTxPath, inputTxPathFlag, "", "Path to the transaction signed by all signatories")
	return cmd
}

func commitTx(_ *cobra.Command, args []string) error {
	var err error
	if inputTxPath == "" {
		inputTxPath, err = app.Prompt.CaptureExistingFilepath("What is the path to the signed transactions file?")
		if err != nil {
			return err
		}
	}
	tx, err := txutils.LoadFromDisk(inputTxPath)
	if err != nil {
		return err
	}

	network, err := txutils.GetNetwork(tx)
	if err != nil {
		return err
	}

	subnetName := args[0]
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	subnetID := sc.Networks[network.Kind.String()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	controlKeys, _, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}
	subnetAuthKeys, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return err
	}

	if len(remainingSubnetAuthKeys) != 0 {
		signedCount := len(subnetAuthKeys) - len(remainingSubnetAuthKeys)
		ux.Logger.PrintToUser("%d of %d required signatures have been signed.", signedCount, len(subnetAuthKeys))
		subnetcmd.PrintRemainingToSignMsg(subnetName, remainingSubnetAuthKeys, inputTxPath)
		return fmt.Errorf("tx is not fully signed")
	}

	// get kc with some random address, to pass wallet creation checks
	kc := secp256k1fx.NewKeychain()
	_, err = kc.New()
	if err != nil {
		return err
	}

	deployer := subnet.NewPublicDeployer(app, false, kc, network)
	txID, err := deployer.Commit(tx)
	if err != nil {
		return err
	}

	if txutils.IsCreateChainTx(tx) {
		if err := subnetcmd.PrintDeployResults(subnetName, subnetID, txID); err != nil {
			return err
		}
		return app.UpdateSidecarNetworks(&sc, network, subnetID, txID)
	}
	ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", txID)

	return nil
}
