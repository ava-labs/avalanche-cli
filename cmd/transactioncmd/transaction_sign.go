// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package transactioncmd

import (
	"github.com/spf13/cobra"
)

var exportPath string

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

	cmd.Flags().StringVar(&exportPath, "export-path", "", "Path to where the signed transaction should be written to disk")
	return cmd
}

func signTx(cmd *cobra.Command, args []string) error {
	return nil
}
