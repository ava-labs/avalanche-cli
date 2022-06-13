// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"errors"
	"os"

	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a subnet configuration",
	Long:  "The subnet delete command deletes an existing subnet configuration.",
	RunE:  deleteGenesis,
	Args:  cobra.ExactArgs(1),
}

func deleteGenesis(cmd *cobra.Command, args []string) error {
	// TODO sanitize this input
	sidecar := app.GetSidecarPath(args[0])
	genesis := app.GetGenesisPath(args[0])

	if _, err := os.Stat(genesis); err == nil {
		// exists
		os.Remove(genesis)
	} else if errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		app.Log.Error("Specified genesis does not exist")
	} else {
		// Schrodinger: file may or may not exist. See err for details.

		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		return err
	}

	if _, err := os.Stat(sidecar); err == nil {
		// exists
		os.Remove(sidecar)
		ux.Logger.PrintToUser("Deleted subnet")
	} else if errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		app.Log.Error("Specified sidecar does not exist")
	} else {
		// Schrodinger: file may or may not exist. See err for details.

		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		return err
	}
	return nil
}
