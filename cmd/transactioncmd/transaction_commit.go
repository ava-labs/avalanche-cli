// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/spf13/cobra"
)

var (
	txFilePath              string
	mainnet, testnet, local bool
)

// avalanche transaction commit
func newTransactionCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "commit [subnetName]",
		Short:        "commit a transaction",
		Long:         "",
		RunE:         commitTx,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}

	cmd.Flags().StringVar(&txFilePath, "tx-file-path", "", "Path to the completed signed transaction")
	cmd.Flags().BoolVar(&mainnet, "mainnet", false, "Issue the transaction on mainnet")
	cmd.Flags().BoolVar(&testnet, "testnet", false, "Issue the transaction on testnet")
	cmd.Flags().BoolVar(&testnet, "fuji", false, "Issue the transaction on fuji")
	cmd.Flags().BoolVar(&local, "local", false, "Issue the transaction on the local network")
	return cmd
}

func commitTx(cmd *cobra.Command, args []string) error {
	allFlags := []bool{mainnet, testnet, local}
	if !flags.EnsureMutuallyExclusive(allFlags) {
		return errors.New("the flags '--mainnet', '--testnet' (resp. '--fuji') and '--local' are mutually exclusive")
	}
	return nil
}
