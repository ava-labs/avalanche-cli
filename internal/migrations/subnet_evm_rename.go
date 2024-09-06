// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package migrations

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"

	"github.com/ava-labs/avalanche-cli/pkg/application"
)

const oldSubnetEVM = "SubnetEVM"

func migrateSubnetEVMNames(app *application.Avalanche, runner *migrationRunner) error {
	subnetDir := app.GetSubnetDir()
	subnets, err := os.ReadDir(subnetDir)
	if err != nil {
		return err
	}

	for _, subnet := range subnets {
		if !subnet.IsDir() {
			continue
		}
		// disregard any empty subnet directories
		dirName := filepath.Join(subnetDir, subnet.Name())
		dirContents, err := os.ReadDir(dirName)
		if err != nil {
			return err
		}
		if len(dirContents) == 0 {
			continue
		}

		if !app.SidecarExists(subnet.Name()) {
			return fmt.Errorf(
				"subnet %s has inconsistent configuration. there is no %s file present on directory %s. please backup any file and then remove the subnet",
				subnet.Name(),
				constants.SidecarFileName,
				dirName,
			)
		}

		sc, err := app.LoadSidecar(subnet.Name())
		if err != nil {
			return err
		}

		if string(sc.VM) == oldSubnetEVM {
			runner.printMigrationMessage()
			sc.VM = models.SubnetEvm
			if err = app.UpdateSidecar(&sc); err != nil {
				return err
			}
		}
	}
	return nil
}
