// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"fmt"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
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
		Use:   "commit [blockchainName]",
		Short: "commit a transaction",
		Long:  "The transaction commit command commits a transaction by submitting it to the P-Chain.",
		RunE:  commitTx,
		Args:  cobrautils.MaximumNArgs(1),
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

	isCreateChainTx := txutils.IsCreateChainTx(tx)

	isConvertToL1Tx := txutils.IsConvertToL1Tx(tx)
	if isConvertToL1Tx {
		doContinue, err := validateConvertOperation(tx, "commit")
		if err != nil {
			return err
		}
		if !doContinue {
			return nil
		}
	}

	network, err := txutils.GetNetwork(tx)
	if err != nil {
		return err
	}

	subnetID, err := txutils.GetSubnetID(tx)
	if err != nil {
		return err
	}
	var (
		blockchainName string
		sc             models.Sidecar
	)
	if len(args) > 0 {
		blockchainName = args[0]
		// subnet ID from tx is always preferred
		if subnetID == ids.Empty {
			sc, err = app.LoadSidecar(blockchainName)
			if err != nil {
				return err
			}
			subnetID = sc.Networks[network.Name()].SubnetID
			if subnetID == ids.Empty {
				return constants.ErrNoSubnetID
			}
		}
	} else if isCreateChainTx {
		ux.Logger.PrintToUser("Tx is going to create a new blockchain ID but CLI can't locally persist")
		ux.Logger.PrintToUser("the new metadata as no blockchain name was provided.")
		ux.Logger.PrintToUser("If you desire to locally persist the blockchain metadata, please ensure")
		ux.Logger.PrintToUser("that CLI manages the blockchain configuration.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("For that you should use the machine where 'avalanche blockchain create' was")
		ux.Logger.PrintToUser("executed, or use another machine but first follow a export/import procedure using")
		ux.Logger.PrintToUser("'avalanche blockchain export' 'avalanche blockchain import file'")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("In case of continuing without preserving the metadata, please keep a manual record of")
		ux.Logger.PrintToUser("the subnet ID and the new blockchain ID")
		ux.Logger.PrintToUser("")
		yes, err := app.Prompt.CaptureYesNo("Do you want to continue execution without locally preserving metadata?")
		if err != nil {
			return err
		}
		if !yes {
			return nil
		}
	}

	_, controlKeys, _, err := txutils.GetOwners(network, subnetID)
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
		blockchaincmd.PrintRemainingToSignMsg(blockchainName, remainingSubnetAuthKeys, inputTxPath)
		return fmt.Errorf("tx is not fully signed")
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

	if isCreateChainTx {
		if err := blockchaincmd.PrintDeployResults(blockchainName, subnetID, txID); err != nil {
			return err
		}
		if blockchainName != "" {
			return app.UpdateSidecarNetworks(&sc, network, subnetID, txID, "", "", sc.Networks[network.Name()].BootstrapValidators, "", "")
		}
		ux.Logger.PrintToUser("This CLI instance will not locally preserve blockchain metadata.")
		ux.Logger.PrintToUser("Please keep a manual record of the subnet ID and blockchain ID information.")
	} else if isConvertToL1Tx {
		_, err = ux.TimedProgressBar(
			30*time.Second,
			"Waiting for the Subnet to be converted into a sovereign L1 ...",
			0,
		)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("To finish conversion to sovereign L1, call `avalanche contract initValidatorManager %s` to finish conversion to sovereign L1", blockchainName)
	}

	return nil
}
