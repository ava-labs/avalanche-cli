// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
)

func UpgradeVM(app *application.Avalanche, vmID string, vmBinPath string) error {
	installer := NewPluginBinaryDownloader(app)
	if err := installer.UpgradeVM(vmID, vmBinPath); err != nil {
		return fmt.Errorf("failed to upgrade vm: %w", err)
	}

	return nil
}
