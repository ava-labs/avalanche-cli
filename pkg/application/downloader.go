// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package application

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

type Downloader interface {
	Download(url string) ([]byte, error)
	GetLatestReleaseVersion(releaseURL string) (string, error)
}

type downloader struct{}

func NewDownloader() Downloader {
	return &downloader{}
}

func (downloader) Download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GetLatestReleaseVersion returns the latest available version from github
func (downloader) GetLatestReleaseVersion(releaseURL string) (string, error) {
	// TODO: Question if there is a less error prone (= simpler) way to install latest avalanchego
	// Maybe the binary package manager should also allow the actual avalanchego binary for download
	request, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request for latest version from %s: %w", releaseURL, err)
	}
	token := os.Getenv(constants.GithubAPITokenEnvVarName)
	if token != "" {
		// avoid rate limitation issues at CI
		request.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
	}
	resp, err := http.DefaultClient.Do(request)
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
