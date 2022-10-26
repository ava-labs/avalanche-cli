// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/storage"
	"github.com/spf13/cobra"
)

// avalanche subnet upgrade import
func newUpgradeExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [subnetName]",
		Short: "Generate the configuration file to upgrade subnet nodes",
		Long:  `Upgrades to subnet nodes can be executed by providing a upgrade.json file to the nodes. This command starts a wizard guiding the user generating the required file.`,
		RunE:  upgradeExportCmd,
		Args:  cobra.ExactArgs(1),
	}

	cmd.Flags().StringVar(&upgradeBytesFilePath, upgradeBytesFilePathKey, "", "Export upgrade bytes file to location of choice on disk")
	cmd.MarkFlagRequired(upgradeBytesFilePathKey)

	return cmd
}

func upgradeExportCmd(cmd *cobra.Command, args []string) error {
	subnetName := args[0]
	if !app.GenesisExists(subnetName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", subnetName)
		return nil
	}

	if _, err := os.Stat(upgradeBytesFilePath); err == nil {
		ux.Logger.PrintToUser("The file specified with path %q already exists!", upgradeBytesFilePath)

		yes, err := app.Prompt.CaptureYesNo("Should we overwrite it?")
		if err != nil {
			return err
		}
		if !yes {
			ux.Logger.PrintToUser("Aborted by user. Nothing has been exported")
			return nil
		}
	}

	subnetPath := filepath.Join(app.GetUpgradeFilesDir(), subnetName)
	localUpgradeBytesFileName := filepath.Join(subnetPath, constants.UpdateBytesFileName)

	exists, err := storage.FileExists(localUpgradeBytesFileName)
	if err != nil {
		return fmt.Errorf("failed to access the upgrade bytes file on the local environment: %w", err)
	}
	if !exists {
		return errors.New("we could not find the upgrade bytes file on the local environment - sure it exists?")
	}

	fileBytes, err := os.ReadFile(localUpgradeBytesFileName)
	if err != nil {
		return fmt.Errorf("failed to read the upgrade bytes file from the local environment: %w", err)
	}

	ux.Logger.PrintToUser("Writing the upgrade bytes file to %q...", upgradeBytesFilePath)
	err = os.WriteFile(upgradeBytesFilePath, fileBytes, constants.DefaultPerms755)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("File written successfully.")
	return nil
}
