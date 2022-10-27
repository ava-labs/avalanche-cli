package upgrades

import (
	"errors"
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

func ReadUpgradeFile(subnetName, upgradeFilesDir string) ([]byte, error) {
	subnetPath := filepath.Join(upgradeFilesDir, subnetName)
	localUpgradeBytesFileName := filepath.Join(subnetPath, constants.UpdateBytesFileName)

	exists, err := storage.FileExists(localUpgradeBytesFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to access the upgrade bytes file on the local environment: %w", err)
	}
	if !exists {
		return nil, errors.New("we could not find the upgrade bytes file on the local environment - sure it exists?")
	}

	fileBytes, err := os.ReadFile(localUpgradeBytesFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read the upgrade bytes file from the local environment: %w", err)
	}
	return fileBytes, nil
}
