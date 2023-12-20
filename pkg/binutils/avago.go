// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func SetupAvalanchego(app *application.Avalanche, avagoVersion string) (string, string, error) {
	binDir := app.GetAvalanchegoBinDir()

	installer := NewInstaller()
	downloader := NewAvagoDownloader()
	return InstallBinary(
		app,
		avagoVersion,
		binDir,
		binDir,
		avalanchegoBinPrefix,
		constants.AvaLabsOrg,
		constants.AvalancheGoRepoName,
		downloader,
		installer,
	)
}
