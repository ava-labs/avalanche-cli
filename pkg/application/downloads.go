// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"golang.org/x/mod/semver"
)

const githubVersionTagName = "tag_name"

// GetLatestPreReleaseVersion returns the latest available pre release version from github
func GetLatestPreReleaseVersion(org, repo string) (string, error) {
	releases, err := GetAllReleasesForRepo(org, repo)
	if err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("no releases found for org %s repo %s", org, repo)
	}
	return releases[0], nil
}

func GetAllReleasesForRepo(org, repo string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", org, repo)
	token := os.Getenv(constants.GithubAPITokenEnvVarName)
	jsonBytes, err := utils.MakeGetRequest(context.Background(), url, token)
	if err != nil {
		return nil, err
	}

	var releaseArr []map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &releaseArr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal binary json version string: %w", err)
	}

	releases := make([]string, len(releaseArr))
	for i, r := range releaseArr {
		version := r[githubVersionTagName].(string)
		if !semver.IsValid(version) {
			return nil, fmt.Errorf("invalid version string: %s", version)
		}
		releases[i] = version
	}

	return releases, nil
}

// GetLatestReleaseVersion returns the latest available release version from github
func GetLatestReleaseVersion(releaseURL string) (string, error) {
	// TODO: Question if there is a less error prone (= simpler) way to install latest avalanchego
	// Maybe the binary package manager should also allow the actual avalanchego binary for download
	token := os.Getenv(constants.GithubAPITokenEnvVarName)
	jsonBytes, err := utils.MakeGetRequest(context.Background(), releaseURL, token)
	if err != nil {
		return "", err
	}

	var jsonStr map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &jsonStr); err != nil {
		return "", fmt.Errorf("failed to unmarshal binary json version string: %w", err)
	}

	version := jsonStr[githubVersionTagName].(string)
	if !semver.IsValid(version) {
		return "", fmt.Errorf("invalid version string: %s", version)
	}

	return version, nil
}
