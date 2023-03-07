// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package migrations

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/models"

	"github.com/ava-labs/avalanche-cli/pkg/application"
)

const oldSubnetEVM = "SubnetEVM"

func migrateSubnetEVMNames(app *application.Avalanche, runner *migrationRunner) error {
	subnetsDir := app.GetSubnetDir()
	subnets, err := os.ReadDir(subnetsDir)
	if err != nil {
		return err
	}

	for _, subnet := range subnets {
		subnetDir := filepath.Join(subnetsDir, subnet.Name())
		fileInfo, err := os.Stat(subnetDir)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			// disregard any empty subnet directories
			dirContents, err := os.ReadDir(subnetDir)
			if err != nil {
				return err
			}
			if len(dirContents) == 0 {
				continue
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
	}
	return nil
}
