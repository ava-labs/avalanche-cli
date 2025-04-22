// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"fmt"
	"os"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var inputUpgradeBytesFilePath string

const upgradeBytesFilePathKey = "upgrade-filepath"

// avalanche blockchain upgrade import
func newUpgradeImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import [blockchainName]",
		Short: "Import the upgrade bytes file into the local environment",
		Long:  `Import the upgrade bytes file into the local environment`,
		RunE:  upgradeImportCmd,
		Args:  cobrautils.ExactArgs(1),
	}

	cmd.Flags().StringVar(&inputUpgradeBytesFilePath, upgradeBytesFilePathKey, "", "Import upgrade bytes file into local environment")

	return cmd
}

func upgradeImportCmd(_ *cobra.Command, args []string) error {
	blockchainName := args[0]
	if !app.GenesisExists(blockchainName) {
		ux.Logger.PrintToUser("The provided blockchain name %q does not exist", blockchainName)
		return nil
	}

	if inputUpgradeBytesFilePath == "" {
		var err error
		inputUpgradeBytesFilePath, err = app.Prompt.CaptureExistingFilepath("Provide the path to the upgrade file to import")
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(inputUpgradeBytesFilePath); err != nil {
		if err == os.ErrNotExist {
			return fmt.Errorf("the upgrade file specified with path %q does not exist", inputUpgradeBytesFilePath)
		}
		return err
	}

	fileBytes, err := os.ReadFile(inputUpgradeBytesFilePath)
	if err != nil {
		return fmt.Errorf("failed to read the provided upgrade file: %w", err)
	}

	upgradeBytesFilePath := app.GetUpgradeBytesFilePath(blockchainName)
	if utils.FileExists(upgradeBytesFilePath) {
		timestamp := time.Now().UTC().Format("20060102150405")
		renamedUpgradeBytesFilePath := upgradeBytesFilePath + "_" + timestamp
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("A previous upgrade is found. Renaming it to %s", renamedUpgradeBytesFilePath)
		if err := os.Rename(upgradeBytesFilePath, renamedUpgradeBytesFilePath); err != nil {
			return err
		}
	}

	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Writing upgrade into %s", upgradeBytesFilePath)

	return app.WriteUpgradeFile(blockchainName, fileBytes)
}
