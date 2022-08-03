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

// func installAvalancheGoWithVersion(app *application.Avalanche, avagoVersion string, installer Installer) (string, error) {
// 	ux.Logger.PrintToUser("Installing avalanchego " + avagoVersion + "...")

// 	avalanchegoURL, ext, err := getAvalancheGoURL(avagoVersion, installer)
// 	if err != nil {
// 		return "", fmt.Errorf("unable to determine avalanchego install URL: %d", err)
// 	}

// 	app.Log.Debug("starting download from %s...", avalanchegoURL)
// 	archive, err := installer.DownloadRelease(avalanchegoURL)
// 	if err != nil {
// 		return "", fmt.Errorf("unable to download avalanchego: %d", err)
// 	}

// 	app.Log.Debug("download successful. installing archive...")
// 	binDir := app.GetAvalanchegoBinDir()
// 	if err := InstallArchive(ext, archive, binDir); err != nil {
// 		return "", err
// 	}
// 	avagoSubDir := "avalanchego-" + avagoVersion
// 	if ext == zipExtension {
// 		// zip contains a build subdir instead of the avagoSubDir expected from tar.gz
// 		if err := os.Rename(filepath.Join(binDir, "build"), filepath.Join(binDir, avagoSubDir)); err != nil {
// 			return "", err
// 		}
// 	}
// 	ux.Logger.PrintToUser("Avalanchego installation successful")
// 	return filepath.Join(binDir, avagoSubDir), nil
// }

func SetupAvalanchego(app *application.Avalanche, avagoVersion string) (string, error) {
	// if avagoVersion == "" {
	// 	// get latest version
	// 	var err error
	// 	avagoVersion, err = GetLatestReleaseVersion(GetGithubLatestReleaseURL(
	// 		constants.AvaLabsOrg,
	// 		constants.AvalancheGoRepoName,
	// 	))
	// 	if err != nil {
	// 		return "", err
	// 	}
	// } else if avagoVersion[0] != 'v' {
	// 	return "", fmt.Errorf(
	// 		"invalid version string. Version must start with v, ex: v1.7.14: %s", avagoVersion)
	// }

	// binChecker := NewBinaryChecker()
	// binDir := app.GetAvalanchegoBinDir()
	// exists, avagoDir, err := binChecker.ExistsWithVersion(binDir, avalanchegoBinPrefix, avagoVersion)
	// if err != nil {
	// 	return "", fmt.Errorf("failed trying to locate avalanchego binary: %s", binDir)
	// }
	// if exists {
	// 	app.Log.Debug("avalanchego " + avagoVersion + " found. Skipping installation")
	// 	return avagoDir, nil
	// }

	// app.Log.Info("Using Avalanchego version: %s", avagoVersion)

	// installer := NewInstaller()
	// return installAvalancheGoWithVersion(app, avagoVersion, installer)

	// Check if already installed
	binDir := app.GetAvalanchegoBinDir()
	binChecker := NewBinaryChecker()
	exists, avalanchegoDir, err := binChecker.ExistsWithVersion(binDir, avalanchegoBinPrefix, avagoVersion)
	if err != nil {
		return "", fmt.Errorf("failed trying to locate avalanchego binary: %s", binDir)
	}
	if exists {
		app.Log.Debug("avalanchego " + avagoVersion + " found. Skipping installation")
		return avalanchegoDir, nil
	}

	installer := NewInstaller()
	downloader := NewAvagoDownloader()
	baseDir, err := InstallBinary(app, avagoVersion, binDir, avalanchegoBinPrefix, constants.AvaLabsOrg, constants.AvalancheGoRepoName, downloader, installer)
	// returnDir := filepath.Join(baseDir, avalanchegoBinPrefix+avagoVersion)
	fmt.Println("Installed to:", baseDir)
	return baseDir, err
}
