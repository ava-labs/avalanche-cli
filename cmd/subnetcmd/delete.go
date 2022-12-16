// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/models"
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

func deleteSubnet(_ *cobra.Command, args []string) error {
	// TODO sanitize this input
	subnetName := args[0]
	subnetDir := filepath.Join(app.GetSubnetDir(), subnetName)

	customVMPath := app.GetCustomVMPath(subnetName)

	sidecar, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	if sidecar.VM == models.CustomVM {
		if _, err := os.Stat(customVMPath); err != nil {
			return err
		}

		// exists
		if err := os.Remove(customVMPath); err != nil {
			return err
		}
	}

	// TODO this method does not delete the imported VM binary if this
	// is an APM subnet. We can't naively delete the binary because it
	// may be used by multiple subnets. We should delete this binary,
	// but only if no other subnet is using it.
	// More info: https://github.com/ava-labs/avalanche-cli/issues/246

	if _, err := os.Stat(subnetDir); err != nil {
		return err
	}

	// exists
	if err := os.RemoveAll(subnetDir); err != nil {
		return err
	}
	return nil
}
