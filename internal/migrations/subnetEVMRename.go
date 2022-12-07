// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package migrations

import (
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
)

const oldSubnetEVM = "SubnetEVM"

func migrateSubnetEVMNames(app *application.Avalanche, runner *migrationRunner) error {
	subnetDir := app.GetSubnetDir()
	subnets, err := os.ReadDir(subnetDir)
	if err != nil {
		return err
	}

	for _, subnet := range subnets {
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
