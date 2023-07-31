// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"os"

	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [keyName]",
		Short: "Exports a signing key",
		Long: `The key export command exports a created signing key. You can use an exported key in other
applications or import it into another instance of Avalanche-CLI.

By default, the tool writes the hex encoded key to stdout. If you provide the --output
flag, the command writes the key to a file of your choosing.`,
		Args:         cobra.ExactArgs(1),
		RunE:         exportKey,
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(
		&filename,
		"output",
		"o",
		"",
		"write the key to the provided file path",
	)

	return cmd
}

func exportKey(_ *cobra.Command, args []string) error {
	keyName := args[0]

	keyPath := app.GetKeyPath(keyName)
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return err
	}

	if filename == "" {
		fmt.Println(string(keyBytes))
		return nil
	}

	return os.WriteFile(filename, keyBytes, constants.WriteReadReadPerms)
}
