// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

// avalanche subnet delete
func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Delete a subnet configuration",
		Long:  "The subnet delete command deletes an existing subnet configuration.",
		RunE:  deleteSubnet,
		Args:  cobra.ExactArgs(1),
	}
}

func deleteSubnet(cmd *cobra.Command, args []string) error {
	// TODO sanitize this input
	sidecarPath := app.GetSidecarPath(args[0])
	genesisPath := app.GetGenesisPath(args[0])
	customVMPath := app.GetCustomVMPath(args[0])

	sidecar, err := app.LoadSidecar(args[0])
	if err != nil {
		return err
	}

	if sidecar.VM == models.CustomVM {
		if _, err := os.Stat(customVMPath); err == nil {
			// exists
			os.Remove(customVMPath)
		} else {
			return err
		}
	}

	// TODO this method does not delete the imported VM binary if this
	// is an APM subnet. We can't naively delete the binary because it
	// may be used by multiple subnets. We should delete this binary,
	// but only if no other subnet is using it.
	// More info: https://github.com/ava-labs/avalanche-cli/issues/246

	if _, err := os.Stat(genesisPath); err == nil {
		// exists
		os.Remove(genesisPath)
	} else {
		return err
	}

	if _, err := os.Stat(sidecarPath); err == nil {
		// exists
		os.Remove(sidecarPath)
		ux.Logger.PrintToUser("Deleted subnet")
	} else {
		return err
	}

	return nil
}
