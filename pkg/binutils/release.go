// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func GetGithubLatestReleaseURL(org, repo string) string {
	return "https://api.github.com/repos/" + org + "/" + repo + "/releases/latest"
}

func installBinaryWithVersion(
	app *application.Avalanche,
	version string,
	binDir string,
	binPrefix, org,
	repo string,
	installURL string,
	ext string,
	installer Installer,
) (string, error) {
	ux.Logger.PrintToUser("Installing " + binPrefix + version + "...")

	app.Log.Debug("starting download from %s...", installURL)
	archive, err := installer.DownloadRelease(installURL)
	if err != nil {
		return "", fmt.Errorf("unable to download subnet-evm: %d", err)
	}

	app.Log.Debug("download successful. installing archive...")
	if err := InstallArchive(ext, archive, binDir); err != nil {
		return "", err
	}

	subDir := binPrefix + version
	if ext == zipExtension {
		// zip contains a build subdir instead of the subnetEVMSubDir expected from tar.gz
		if err := os.Rename(filepath.Join(binDir, "build"), filepath.Join(binDir, subDir)); err != nil {
			return "", err
		}
	}
	ux.Logger.PrintToUser(binPrefix + version + " installation successful")
	return filepath.Join(binDir, subDir), nil
}

func InstallBinary(app *application.Avalanche, version string, binDir string, binPrefix, org, repo string) (string, error) {
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

	installer := NewInstaller()
	return installAvalancheGoWithVersion(app, version, installer)
}
