// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package migrations

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
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
			ux.Logger.PrintToUser(
				logging.Yellow.Wrap("blockchain %s has inconsistent configuration. cleaning it up"),
				subnet.Name(),
			)
			if err := blockchaincmd.CallDeleteBlockchain(subnet.Name()); err != nil {
				return err
			}
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
	return nil
}
