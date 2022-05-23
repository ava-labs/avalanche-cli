// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/ux"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes a generated subnet genesis",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: deleteGenesis,
	Args: cobra.ExactArgs(1),
}

func deleteGenesis(cmd *cobra.Command, args []string) error {
	// TODO sanitize this input
	genesis := filepath.Join(baseDir, args[0]+genesis_suffix)
	sidecar := filepath.Join(baseDir, args[0]+sidecar_suffix)

	if _, err := os.Stat(genesis); err == nil {
		// exists
		os.Remove(genesis)
	} else if errors.Is(err, os.ErrNotExist) {
		// does *not* exist
		log.Error("Specified genesis does not exist")
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
		log.Error("Specified sidecar does not exist")
	} else {
		// Schrodinger: file may or may not exist. See err for details.

		// Therefore, do *NOT* use !os.IsNotExist(err) to test for file existence
		return err
	}
	return nil
}
