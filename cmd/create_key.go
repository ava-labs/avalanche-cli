// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/wallet"
	"github.com/spf13/cobra"
)

// createKeyCmd represents the create command
var (
	createKeyCmd = &cobra.Command{
		Use:   "key create ",
		Short: "Create a new private key",
		Long:  ``,
		Args:  cobra.ExactArgs(1),
		RunE:  createKey,
	}
	privKeyPath string
)

func createKey(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(privKeyPath); err == nil {
		// color.Outf("{{red}}key already found at %q{{/}}\n", privKeyPath)
		return os.ErrExist
	}
	k, err := wallet.NewSoft(0)
	if err != nil {
		return err
	}
	if err := k.Save(privKeyPath); err != nil {
		return err
	}
	// color.Outf("{{green}}created a new key %q{{/}}\n", privKeyPath)
	return nil
}
