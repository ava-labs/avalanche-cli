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
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
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
		return "", fmt.Errorf("unable to determine binary install URL: %w", err)
	}

	app.Log.Debug("starting download...", zap.String("download-url", installURL))
	archive, err := app.Downloader.Download(installURL)
	if err != nil {
		return "", fmt.Errorf("unable to download binary: %w", err)
	}

	app.Log.Debug("download successful. installing archive...")
	if err := InstallArchive(ext, archive, binDir); err != nil {
		return "", err
	}

	if ext == zipExtension {
		// zip contains a build subdir instead of the toplevel expected from tar.gz
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
	baseBinDir string,
	installDir string,
	binPrefix,
	org,
	repo string,
	downloader GithubDownloader,
	installer Installer,
) (string, error) {
	if version == "latest" {
		// get latest version
		var err error
		version, err = app.Downloader.GetLatestReleaseVersion(GetGithubLatestReleaseURL(
			org,
			repo,
		))
		if err != nil {
			return "", err
		}
	} else if !semver.IsValid(version) {
		return "", fmt.Errorf(
			"invalid version string. Must be semantic version ex: v1.7.14: %s", version)
	}

	binChecker := NewBinaryChecker()

	exists, err := binChecker.ExistsWithVersion(baseBinDir, binPrefix, version)
	if err != nil {
		return "", fmt.Errorf("failed trying to locate binary %s-%s: %s", binPrefix, version, baseBinDir)
	}
	if exists {
		app.Log.Debug(binPrefix + version + " found. Skipping installation")
		return filepath.Join(baseBinDir, binPrefix+version), nil
	}

	app.Log.Info("Using binary version", zap.String("version", version))

	binDir, err := installBinaryWithVersion(app, version, installDir, binPrefix, downloader, installer)

	return binDir, err
}
