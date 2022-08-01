// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package keycmd

import (
	"errors"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var forceDelete bool

// avalanche key delete
func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [keyName]",
		Short: "Delete a signing key",
		Long: `The key delete command deletes an existing signing key.

To delete a key, provide the keyName. The command will prompt for confirmation
before deleting the key. To skip the confirmation, provide the --force flag.`,
		RunE:         deleteKey,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
	cmd.Flags().BoolVarP(
		&forceDelete,
		forceFlag,
		"f",
		false,
		"delete the key without confirmation",
	)
	return cmd
}

func deleteKey(cmd *cobra.Command, args []string) error {
	keyName := args[0]
	keyPath := app.GetKeyPath(keyName)

	// Check file exists
	_, err := os.Stat(keyPath)
	if err != nil {
		return errors.New("key does not exist")
	}

	if !forceDelete {
		confStr := "Are you sure you want to delete " + keyName + "?"
		conf, err := app.Prompt.CaptureNoYes(confStr)
		if err != nil {
			return err
		}

		if !conf {
			ux.Logger.PrintToUser("Delete cancelled")
			return nil
		}
	}

	// exists
	if err = os.Remove(keyPath); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Key deleted")

	return nil
}
