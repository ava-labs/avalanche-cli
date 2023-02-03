// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/subnet/upgrades"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var upgradeBytesFilePath string

const upgradeBytesFilePathKey = "upgrade-filepath"

// avalanche subnet upgrade import
func newUpgradeImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import [subnetName]",
		Short: "Import the upgrade bytes file into the local environment",
		Long:  `Import the upgrade bytes file into the local environment`,
		RunE:  upgradeImportCmd,
		Args:  cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&upgradeBytesFilePath, upgradeBytesFilePathKey, "", "Import upgrade bytes file into local environment")

	return cmd
}

func upgradeImportCmd(_ *cobra.Command, args []string) error {
	subnetName := args[0]
	if !app.GenesisExists(subnetName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", subnetName)
		return nil
	}

	if upgradeBytesFilePath == "" {
		var err error
		upgradeBytesFilePath, err = app.Prompt.CaptureExistingFilepath("Provide the path to the upgrade file to import")
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(upgradeBytesFilePath); err != nil {
		if err == os.ErrNotExist {
			return fmt.Errorf("the upgrade file specified with path %q does not exist", upgradeBytesFilePath)
		}
		return err
	}

	fileBytes, err := os.ReadFile(upgradeBytesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read the provided upgrade file: %w", err)
	}

	return upgrades.WriteUpgradeFile(fileBytes, subnetName, app)
}
