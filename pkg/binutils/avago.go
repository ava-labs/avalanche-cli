package binutils

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

const (
	zipExtension = "zip"
	tarExtension = "tar.gz"
)

func SetupAvalanchego(app *application.Avalanche, avagoVersion string) (string, error) {
	// Check if already installed
	binDir := app.GetAvalanchegoBinDir()

	installer := NewInstaller()
	downloader := NewAvagoDownloader()
	baseDir, err := InstallBinary(app, avagoVersion, binDir, binDir, avalanchegoBinPrefix, constants.AvaLabsOrg, constants.AvalancheGoRepoName, downloader, installer)
	// returnDir := filepath.Join(baseDir, avalanchegoBinPrefix+avagoVersion)
	fmt.Println("Installed to:", baseDir)
	return baseDir, err
}
