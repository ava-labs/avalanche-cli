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

const (
	githubDraftTagName      = "draft"
	githubPrereleaseTagName = "prerelease"
	githubVersionTagName    = "tag_name"
)

type ReleaseKind int64

const (
	Undefined ReleaseKind = iota
	Prerelease
	Release
	All
)

// This is a generic interface for performing highly testable downloads. All methods here involve
// external http requests. To write tests using these functions, provide a mocked version of this
// interface to your application object.
type Downloader interface {
	Download(url string) ([]byte, error)
	GetLatestReleaseVersion(org, repo, component string) (string, error)
	GetLatestPreReleaseVersion(org, repo, component string) (string, error)
	GetAllReleasesForRepo(org, repo string, kind ReleaseKind) ([]string, error)
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

// GetLatestPreReleaseVersion returns the latest available pre release or release version from github
func (d downloader) GetLatestPreReleaseVersion(org, repo, component string) (string, error) {
	releases, err := d.GetAllReleasesForRepo(org, repo, All)
	if err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("no releases or prereleases found for org %s repo %s", org, repo)
	}
	if component == "" {
		return releases[0], nil
	}
	for _, release := range releases {
		if strings.HasPrefix(release, component) {
			return release, nil
		}
	}
	return "", fmt.Errorf("no releases or prereleases found for org %s repo %s component %s", org, repo, component)
}

// GetLatestReleaseVersion returns the latest available release version from github
func (d downloader) GetLatestReleaseVersion(org, repo, component string) (string, error) {
	if component == "" {
		return d.getLatestReleaseVersion(org, repo)
	}
	releases, err := d.GetAllReleasesForRepo(org, repo, Release)
	if err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("no releases found for org %s repo %s", org, repo)
	}
	for _, release := range releases {
		if strings.HasPrefix(release, component) {
			return release, nil
		}
	}
	return "", fmt.Errorf("no releases found for org %s repo %s component %s", org, repo, component)
}

func (d downloader) GetAllReleasesForRepo(org, repo string, kind ReleaseKind) ([]string, error) {
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
		if isDraft, ok := r[githubDraftTagName].(bool); ok && isDraft {
			continue
		}
		isPrerelease, ok := r[githubPrereleaseTagName].(bool)
		if !ok {
			continue
		}
		if kind == Prerelease && !isPrerelease {
			continue
		}
		if kind == Release && isPrerelease {
			continue
		}
		version := r[githubVersionTagName].(string)
		if !utils.IsValidSemanticVersion(version) {
			// will skip ICM services version format errors until format is firmly defined
			if repo == constants.ICMServicesRepoName {
				continue
			}
			return nil, fmt.Errorf("invalid version string: %s", version)
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

func (d downloader) getLatestReleaseVersion(org, repo string) (string, error) {
	releaseURL := "https://api.github.com/repos/" + org + "/" + repo + "/releases/latest"
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
