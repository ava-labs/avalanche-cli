// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/txutils"
	"github.com/spf13/cobra"
)

const inputTxPathFlag = "input-tx-filepath"

var inputTxPath string

// avalanche transaction sign
func newTransactionSignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sign",
		Short:        "sign a transaction",
		Long:         "",
		RunE:         signTx,
		Args:         cobra.ExactArgs(0),
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&inputTxPath, inputTxPathFlag, "", "Path to the transaction file for signing")
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
	network, err := txutils.GetNetwork(tx)
	if err != nil {
		return err
	}
	fmt.Println(network)
	return nil
}
