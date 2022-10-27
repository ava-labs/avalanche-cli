// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"bytes"
	"encoding/json"
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
func newUpgradePrintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print [subnetName]",
		Short: "Print the upgrade.json file content",
		Long:  `Print the upgrade.json file content`,
		RunE:  upgradePrintCmd,
		Args:  cobra.ExactArgs(1),
	}

	return cmd
}

func upgradePrintCmd(cmd *cobra.Command, args []string) error {
	subnetName := args[0]
	if !app.GenesisExists(subnetName) {
		ux.Logger.PrintToUser("The provided subnet name %q does not exist", subnetName)
		return nil
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

	var prettyJSON bytes.Buffer
	if err = json.Indent(&prettyJSON, fileBytes, "", "  "); err != nil {
		return err
	}
	ux.Logger.PrintToUser(prettyJSON.String())
	return nil
}
