// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"go.uber.org/zap"
)

// CheckReleaseVersion checks the latest available version from github for the given repo and version
// and returns an http response for it in success case
//
// NOTE: If any of the underlying URLs change (github changes, release file names, etc.) this fails
// The goal MUST be to have some sort of mature binary management
func CheckReleaseVersion(
	log logging.Logger,
	repo string,
	version string,
) (*http.Response, error) {
	arch := runtime.GOARCH
	goos := runtime.GOOS
	var downloadURL string

	switch goos {
	case "linux":
		downloadURL = fmt.Sprintf(
			"https://github.com/ava-labs/%s/releases/download/%s/%s_%s_linux_%s.tar.gz",
			repo,
			version,
			repo,
			version[1:], // WARN subnet-evm isn't consistent in its release naming, it's omitting the v in the file name...
			arch,
		)
	case "darwin":
		downloadURL = fmt.Sprintf(
			"https://github.com/ava-labs/%s/releases/download/%s/%s_%s_darwin_%s.tar.gz",
			repo,
			version,
			repo,
			version[1:],
			arch,
		)
	default:
		return nil, fmt.Errorf("OS not supported: %s", goos)
	}

	log.Debug("starting download...", zap.String("download-url", downloadURL))

	return prompts.RequestURL(downloadURL)
}

// DownloadReleaseVersion returns the latest available version from github for
// the given repo and version, and installs it into the apps `bin` dir.
// NOTE: If any of the underlying URLs change (github changes, release file names, etc.) this fails
// The goal MUST be to have some sort of mature binary management
//
// Deprecated: Use GetLatestReleaseVersion
func DownloadReleaseVersion(
	log logging.Logger,
	repo,
	version,
	binDir string,
) (string, error) {
	resp, err := CheckReleaseVersion(log, repo, version)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	archive, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	installDir := filepath.Join(binDir, repo+"-"+version)
	if err := os.MkdirAll(installDir, perms.ReadWriteExecute); err != nil {
		return "", fmt.Errorf("failed creating %s installation directory: %w", repo, err)
	}

	log.Debug("download successful. installing archive...")
	if err := InstallArchive("tar.gz", archive, installDir); err != nil {
		return "", err
	}
	return installDir, nil
}
