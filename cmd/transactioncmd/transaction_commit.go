// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"github.com/spf13/cobra"
)

var txFilePath string

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
	return cmd
}

func commitTx(cmd *cobra.Command, args []string) error {
	return nil
}
