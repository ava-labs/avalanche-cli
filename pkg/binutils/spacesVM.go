// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func SetupSpacesVM(app *application.Avalanche, spacesVMVersion string) (string, error) {
	// Check if already installed
	binDir := app.GetSpacesVMBinDir()
	subDir := filepath.Join(binDir, spacesVMBinPrefix+spacesVMVersion)

	installer := NewInstaller()
	downloader := NewSpacesVMDownloader()
	vmDir, err := InstallBinary(
		app,
		spacesVMVersion,
		binDir,
		subDir,
		spacesVMBinPrefix,
		constants.AvaLabsOrg,
		constants.SpacesVMRepoName,
		downloader,
		installer,
	)
	return filepath.Join(vmDir, constants.SpacesVMBin), err
}
