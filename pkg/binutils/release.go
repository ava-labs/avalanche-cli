// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func installBinaryWithVersion(
	app *application.Avalanche,
	version string,
	binDir string,
	binPrefix string,
	downloader GithubDownloader,
	installer Installer,
) (string, error) {
	ux.Logger.PrintToUser("Installing " + binPrefix + version + "...")

	installURL, ext, err := downloader.GetDownloadURL(version, installer)
	if err != nil {
		return "", fmt.Errorf("unable to determine avalanchego install URL: %d", err)
	}

	app.Log.Debug("starting download from %s ...", installURL)
	archive, err := installer.DownloadRelease(installURL)
	if err != nil {
		return "", fmt.Errorf("unable to download subnet-evm: %d", err)
	}

	app.Log.Debug("download successful. installing archive...")
	if err := InstallArchive(ext, archive, binDir); err != nil {
		fmt.Println("Returning early with err", err)
		return "", err
	}

	fmt.Println("Finished installing archive")

	if ext == zipExtension {
		// zip contains a build subdir instead of the subnetEVMSubDir expected from tar.gz
		// TODO definitely test this
		if err := os.Rename(filepath.Join(binDir, "build"), filepath.Join(binDir, binPrefix+version)); err != nil {
			return "", err
		}
	}
	ux.Logger.PrintToUser(binPrefix + version + " installation successful")

	if !strings.Contains(binDir, version) {
		return filepath.Join(binDir, binPrefix+version), nil
	}

	return binDir, nil
}

func InstallBinary(
	app *application.Avalanche,
	version string,
	binDir string,
	binPrefix,
	org,
	repo string,
	downloader GithubDownloader,
	installer Installer,
) (string, error) {

	fmt.Println("Bin dir", binDir)

	if version == "" {
		// get latest version
		var err error
		version, err = GetLatestReleaseVersion(GetGithubLatestReleaseURL(
			org,
			repo,
		))
		if err != nil {
			return "", err
		}
	} else if version[0] != 'v' {
		return "", fmt.Errorf(
			"invalid version string. Version must start with v, ex: v1.7.14: %s", version)
	}

	binChecker := NewBinaryChecker()

	exists, installDir, err := binChecker.ExistsWithVersion(binDir, binPrefix, version)
	if err != nil {
		return "", fmt.Errorf("failed trying to locate binary %s-%s: %s", binPrefix, version, binDir)
	}
	if exists {
		app.Log.Debug(binPrefix + version + " found. Skipping installation")
		return installDir, nil
	}

	app.Log.Info("Using binary version: %s", version)

	return installBinaryWithVersion(app, version, binDir, binPrefix, downloader, installer)
}
