// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package application

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

const githubVersionTagName = "tag_name"

// This is a generic interface for performing highly testable downloads. All methods here involve
// external http requests. To write tests using these functions, provide a mocked version of this
// interface to your application object.
type Downloader interface {
	Download(url string) ([]byte, error)
	GetLatestReleaseVersion(releaseURL string) (string, error)
	GetLatestPreReleaseVersion(org, repo, kind string) (string, error)
	GetAllReleasesForRepo(org, repo string) ([]string, error)
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

// GetLatestPreReleaseVersion returns the latest available pre release version from github
func (d downloader) GetLatestPreReleaseVersion(org, repo, kind string) (string, error) {
	releases, err := d.GetAllReleasesForRepo(org, repo)
	if err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("no releases found for org %s repo %s", org, repo)
	}
	if kind == "" {
		return releases[0], nil
	}
	for _, release := range releases {
		if strings.HasPrefix(release, kind) {
			return release, nil
		}
	}
	return "", fmt.Errorf("no releases found for org %s repo %s kind %s", org, repo, kind)
}

func (d downloader) GetAllReleasesForRepo(org, repo string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", org, repo)
	token := os.Getenv(constants.GithubAPITokenEnvVarName)
	body, err := d.doAPIRequest(url, token)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	jsonBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest binary version from %s: %w", url, err)
	}

	var releaseArr []map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &releaseArr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal binary json version string: %w", err)
	}

	releases := make([]string, len(releaseArr))
	for i, r := range releaseArr {
		version := r[githubVersionTagName].(string)
		if !utils.IsValidSemanticVersion(version) {
			if repo != constants.ICMServicesRepoName {
				return nil, fmt.Errorf("invalid version string: %s", version)
			}
		}
		releases[i] = version
	}

	return releases, nil
}

func (downloader) doAPIRequest(url, token string) (io.ReadCloser, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", url, err)
	}
	if token != "" {
		// avoid rate limitation issues at CI
		request.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed doing request to %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed doing request %s: unexpected http status code: %d", url, resp.StatusCode)
	}
	return resp.Body, nil
}

// GetLatestReleaseVersion returns the latest available release version from github
func (d downloader) GetLatestReleaseVersion(releaseURL string) (string, error) {
	// TODO: Question if there is a less error prone (= simpler) way to install latest avalanchego
	// Maybe the binary package manager should also allow the actual avalanchego binary for download
	token := os.Getenv(constants.GithubAPITokenEnvVarName)
	body, err := d.doAPIRequest(releaseURL, token)
	if err != nil {
		return "", err
	}
	defer body.Close()

	jsonBytes, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("failed to get latest binary version from %s: %w", releaseURL, err)
	}

	var jsonStr map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &jsonStr); err != nil {
		return "", fmt.Errorf("failed to unmarshal binary json version string: %w", err)
	}

	version := jsonStr[githubVersionTagName].(string)
	if !utils.IsValidSemanticVersion(version) {
		return "", fmt.Errorf("invalid version string: %s", version)
	}

	return version, nil
}
