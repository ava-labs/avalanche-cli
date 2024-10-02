// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

// avalanche transaction commit
func newTransactionCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit [subnetName]",
		Short: "commit a transaction",
		Long:  "The transaction commit command commits a transaction by submitting it to the P-Chain.",
		RunE:  commitTx,
		Args:  cobrautils.ExactArgs(1),
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
	subnetID := sc.Networks[network.Name()].SubnetID
	if subnetID == ids.Empty {
		return errNoSubnetID
	}

	isPermissioned, controlKeys, _, err := txutils.GetOwners(network, subnetID)
	if err != nil {
		return err
	}
	if !isPermissioned {
		return blockchaincmd.ErrNotPermissionedSubnet
	}
	subnetAuthKeys, remainingSubnetAuthKeys, err := txutils.GetRemainingSigners(tx, controlKeys)
	if err != nil {
		return err
	}

	if len(remainingSubnetAuthKeys) != 0 {
		signedCount := len(subnetAuthKeys) - len(remainingSubnetAuthKeys)
		ux.Logger.PrintToUser("%d of %d required signatures have been signed.", signedCount, len(subnetAuthKeys))
		blockchaincmd.PrintRemainingToSignMsg(subnetName, remainingSubnetAuthKeys, inputTxPath)
		return errors.New("tx is not fully signed")
	}

	// get kc with some random address, to pass wallet creation checks
	kc := secp256k1fx.NewKeychain()
	_, err = kc.New()
	if err != nil {
		return err
	}

	deployer := subnet.NewPublicDeployer(app, keychain.NewKeychain(network, kc, nil, nil), network)
	txID, err := deployer.Commit(tx, true)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("Transaction successful, transaction ID: %s", txID)

	if txutils.IsCreateChainTx(tx) {
		// TODO: teleporter for multisig
		if err := blockchaincmd.PrintDeployResults(subnetName, subnetID, txID); err != nil {
			return err
		}
		return app.UpdateSidecarNetworks(&sc, network, subnetID, txID, "", "")
	}

	return nil
}
