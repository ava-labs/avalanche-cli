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

func WriteLockUpgradeFile(jsonBytes []byte, subnetName string, app *application.Avalanche) error {
	upgradeLockBytesFilePath := app.GetUpgradeBytesFilePath(subnetName) + constants.UpgradeBytesLockExtension

	return writeFile(jsonBytes, upgradeLockBytesFilePath)
}

func WriteUpgradeFile(jsonBytes []byte, subnetName string, app *application.Avalanche) error {
	upgradeBytesFilePath := app.GetUpgradeBytesFilePath(subnetName)

	ux.Logger.PrintToUser(fmt.Sprintf("Writing file %q...", upgradeBytesFilePath))
	if err := writeFile(jsonBytes, upgradeBytesFilePath); err != nil {
		return err
	}

	ux.Logger.PrintToUser("File written successfully")
	return nil
}

func writeFile(jsonBytes []byte, path string) error {
	var (
		exists bool
		err    error
	)

	// NOTE: This allows creating the update bytes file before a subnet has actually been created.
	// It is probably never going to happen though, as commands calling this will
	// check if the subnet exists before this
	subnetDir := filepath.Base(path)
	exists, err = storage.FolderExists(subnetDir)
	if err != nil {
		return err
	}
	if !exists {
		if err := os.Mkdir(subnetDir, constants.DefaultPerms755); err != nil {
			return err
		}
	}

	return os.WriteFile(path, jsonBytes, constants.DefaultPerms755)
}

func ReadUpgradeFile(subnetName string, app *application.Avalanche) ([]byte, error) {
	localUpgradeBytesFilePath := app.GetUpgradeBytesFilePath(subnetName)

	return readFile(localUpgradeBytesFilePath)
}

func ReadLockUpgradeFile(subnetName string, app *application.Avalanche) ([]byte, error) {
	localLockUpgradeBytesFilePath := app.GetUpgradeBytesFilePath(subnetName) + constants.UpgradeBytesLockExtension

	return readFile(localLockUpgradeBytesFilePath)
}

func readFile(path string) ([]byte, error) {
	exists, err := storage.FileExists(path)
	if err != nil {
		return nil, fmt.Errorf("failed to access the upgrade bytes file on the local environment: %w", err)
	}
	if !exists {
		return nil, errors.New("we could not find the upgrade bytes file on the local environment - sure it exists?")
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read the upgrade bytes file from the local environment: %w", err)
	}
	return fileBytes, nil
}
