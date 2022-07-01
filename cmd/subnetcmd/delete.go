// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	return deleteCmd
}

// avalanche subnet delete
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
	} else {
		return err
	}

	if _, err := os.Stat(sidecar); err == nil {
		// exists
		os.Remove(sidecar)
		ux.Logger.PrintToUser("Deleted subnet")
	} else {
		return err
	}
	return nil
}
