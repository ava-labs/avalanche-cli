// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgrades

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/storage"
)

func WriteUpgradeFile(jsonBytes []byte, subnetName string, app *application.Avalanche) error {
	var (
		exists bool
		err    error
	)

	upgradeBytesFilePath := app.GetUpgradeBytesFilePath(subnetName)
	ux.Logger.PrintToUser(fmt.Sprintf("Writing file %q...", upgradeBytesFilePath))

	// NOTE: This allows creating the update bytes file before a subnet has actually been created.
	// It is probably never going to happen though, as commands calling this will
	// check if the subnet exists before this
	subnetDir := filepath.Base(upgradeBytesFilePath)
	exists, err = storage.FolderExists(subnetDir)
	if err != nil {
		return err
	}
	if !exists {
		if err := os.Mkdir(subnetDir, constants.DefaultPerms755); err != nil {
			return err
		}
	}

	if err = os.WriteFile(upgradeBytesFilePath, jsonBytes, constants.DefaultPerms755); err != nil {
		return err
	}
	ux.Logger.PrintToUser("File written successfully")
	return nil
}

func ReadUpgradeFile(subnetName string, app *application.Avalanche) ([]byte, error) {
	localUpgradeBytesFilePath := app.GetUpgradeBytesFilePath(subnetName)

	exists, err := storage.FileExists(localUpgradeBytesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to access the upgrade bytes file on the local environment: %w", err)
	}
	if !exists {
		return nil, errors.New("we could not find the upgrade bytes file on the local environment - sure it exists?")
	}

	fileBytes, err := os.ReadFile(localUpgradeBytesFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the upgrade bytes file from the local environment: %w", err)
	}
	return fileBytes, nil
}
