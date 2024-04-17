// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/spf13/cobra"
)

// avalanche subnet delete
func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [subnetName]",
		Short: "Delete a subnet configuration",
		Long:  "The subnet delete command deletes an existing subnet configuration.",
		RunE:  deleteSubnet,
		Args:  cobrautils.ExactArgs(1),
	}
}

func deleteSubnet(_ *cobra.Command, args []string) error {
	// TODO sanitize this input
	subnetName := args[0]

	sidecar, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}

	if sidecar.VM == models.CustomVM {
		customVMPath := app.GetCustomVMPath(subnetName)
		if _, err := os.Stat(customVMPath); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			app.Log.Warn("tried to remove custom VM path but it actually does not exist. Ignoring")
			return nil
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

	subnetDir := filepath.Join(app.GetSubnetDir(), subnetName)
	if _, err := os.Stat(subnetDir); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		app.Log.Warn("tried to remove the Subnet dir path but it actually does not exist. Ignoring")
		return nil
	}

	// rm airdrop key if exists
	airdropKeyName, _, _, err := subnet.GetSubnetAirdropKeyInfo(app, subnetName)
	if err != nil {
		return err
	}
	if airdropKeyName != "" {
		airdropKeyPath := app.GetKeyPath(airdropKeyName)
		if err := os.Remove(airdropKeyPath); err != nil {
			return err
		}
	}

	// exists
	if err := os.RemoveAll(subnetDir); err != nil {
		return err
	}
	return nil
}
