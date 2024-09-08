// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"encoding/json"
	"fmt"

	"golang.org/x/mod/semver"
)

const githubVersionTagName = "tag_name"

func GetLatestGithubReleaseURL(org, repo string) string {
	return fmt.Sprintf("%s/%s", GetGithubReleasesURL(org, repo), "latest")
}

func GetGithubReleasesURL(org, repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", org, repo)
}

// GetLatestGithubReleaseVersion returns the latest available release version from github
func GetLatestGithubReleaseVersion(org, repo, authToken string) (string, error) {
	url := GetLatestGithubReleaseURL(org, repo)
	jsonBytes, err := HTTPGet(url, authToken)
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

func GetAllGithubReleaseVersions(org, repo, authToken string) ([]string, error) {
	url := GetGithubReleasesURL(org, repo)
	jsonBytes, err := HTTPGet(url, authToken)
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

// GetLatestGithubPreReleaseVersion returns the latest available pre release version from github
func GetLatestGithubPreReleaseVersion(org, repo, authToken string) (string, error) {
	releases, err := GetAllGithubReleaseVersions(org, repo, authToken)
	if err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("no releases found for org %s repo %s", org, repo)
	}
	return releases[0], nil
}

func GetGithubReleaseAssetURL(org, repo, version, asset string) string {
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		org,
		repo,
		version,
		asset,
	)
}
