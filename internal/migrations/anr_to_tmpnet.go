// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package migrations

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/localnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func migrateANRToTmpNet(app *application.Avalanche, _ *migrationRunner) error {
	return localnet.MigrateANRToTmpNet(app, ux.Logger.PrintToUser)
}
