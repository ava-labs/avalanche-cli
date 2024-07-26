// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var force bool

// avalanche blockchain upgrade import
func newUpgradeExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [blockchainName]",
		Short: "Export the upgrade bytes file to a location of choice on disk",
		Long:  `Export the upgrade bytes file to a location of choice on disk`,
		RunE:  upgradeExportCmd,
		Args:  cobrautils.ExactArgs(1),
	}

	cmd.Flags().StringVar(&upgradeBytesFilePath, upgradeBytesFilePathKey, "", "Export upgrade bytes file to location of choice on disk")
	cmd.Flags().BoolVar(&force, "force", false, "If true, overwrite a possibly existing file without prompting")

	return cmd
}

func upgradeExportCmd(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	if !app.GenesisExists(blockchainName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", blockchainName)
		return nil
	}

	if upgradeBytesFilePath == "" {
		var err error
		upgradeBytesFilePath, err = app.Prompt.CaptureString("Provide a path where we should export the file to")
		if err != nil {
			return err
		}
	}

	if !force {
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
	}

	fileBytes, err := app.ReadUpgradeFile(blockchainName)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Writing the upgrade bytes file to %q...", upgradeBytesFilePath)
	err = os.WriteFile(upgradeBytesFilePath, fileBytes, constants.DefaultPerms755)
	if err != nil {
		return err
	}

	ux.Logger.PrintToUser("File written successfully.")
	return nil
}
