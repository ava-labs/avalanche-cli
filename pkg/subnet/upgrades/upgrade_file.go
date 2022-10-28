// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgrades

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/storage"
)

func WriteUpgradeFile(jsonBytes []byte, subnetName, upgradeFilesDir string) error {
	var (
		exists bool
		err    error
	)

	subnetPath := filepath.Join(upgradeFilesDir, subnetName)
	updateBytesFileName := filepath.Join(subnetPath, constants.UpdateBytesFileName)

	ux.Logger.PrintToUser(fmt.Sprintf("Writing %q file to %q...", constants.UpdateBytesFileName, subnetPath))

	exists, err = storage.FolderExists(upgradeFilesDir)
	if err != nil {
		return err
	}
	if !exists {
		if err := os.Mkdir(upgradeFilesDir, constants.DefaultPerms755); err != nil {
			return err
		}
	}

	exists, err = storage.FolderExists(subnetPath)
	if err != nil {
		return err
	}
	if !exists {
		if err := os.Mkdir(subnetPath, constants.DefaultPerms755); err != nil {
			return err
		}
	}

	if err = os.WriteFile(updateBytesFileName, jsonBytes, constants.DefaultPerms755); err != nil {
		return err
	}
	ux.Logger.PrintToUser("File written successfully")
	return nil
}
