// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
)

func GetGithubReleaseURL(org, repo string) string {
	return "https://api.github.com/repos/" + org + "/" + repo + "/releases/latest"
}

// GetLatestReleaseVersion returns the latest available version from github
func GetLatestReleaseVersion(releaseURL string) (string, error) {
	// TODO: Question if there is a less error prone (= simpler) way to install latest avalanchego
	// Maybe the binary package manager should also allow the actual avalanchego binary for download
	resp, err := http.Get(releaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to get latest version from %s: %w", releaseURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get latest version from %s: unexpected http status code: %d", releaseURL, resp.StatusCode)
	}
	defer resp.Body.Close()

	jsonBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to get latest binary version from %s: %w", releaseURL, err)
	}

	var jsonStr map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &jsonStr); err != nil {
		return "", fmt.Errorf("failed to unmarshal binary json version string: %w", err)
	}

	version := jsonStr["tag_name"].(string)
	if version == "" || version[0] != 'v' {
		return "", fmt.Errorf("invalid version string: %s", version)
	}

	return version, nil
}

// DownloadReleaseVersion returns the latest available version from github for
// the given repo and version, and installs it into the apps `bin` dir.
// NOTE: If any of the underlying URLs change (github changes, release file names, etc.) this fails
// The goal MUST be to have some sort of mature binary management
func DownloadReleaseVersion(
	log logging.Logger,
	repo,
	version,
	binDir string,
) (string, error) {
	arch := runtime.GOARCH
	goos := runtime.GOOS
	var downloadURL string
	var ext string

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
		ext = "tar.gz"
	case "darwin":
		downloadURL = fmt.Sprintf(
			"https://github.com/ava-labs/%s/releases/download/%s/%s_%s_darwin_%s.tar.gz",
			repo,
			version,
			repo,
			version[1:],
			arch,
		)
		// subnet-evm supports darwin and linux only
		ext = "tar.gz"
	default:
		return "", fmt.Errorf("OS not supported: %s", goos)
	}

	log.Debug("starting download from %s...", downloadURL)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	archive, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	installDir := filepath.Join(binDir, repo+"-"+version)
	if err := os.MkdirAll(installDir, perms.ReadWriteExecute); err != nil {
		return "", fmt.Errorf("failed creating %s installation directory: %s", repo, err)
	}

	log.Debug("download successful. installing archive...")
	if err := InstallArchive(ext, archive, installDir); err != nil {
		return "", err
	}
	return installDir, nil
}
