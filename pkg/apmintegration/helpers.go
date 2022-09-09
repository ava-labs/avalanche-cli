// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apmintegration

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

func removeSlashes(str string) string {
	return strings.TrimSuffix(strings.TrimPrefix(str, "/"), "/")
}

func getGitOrg(gitURL *url.URL) (string, error) {
	org, repo := path.Split(gitURL.Path)

	org = removeSlashes(org)
	repo = removeSlashes(repo)

	if org == "" || repo == "" {
		return "", errors.New("invalid url format, unable to find org: " + gitURL.Path)
	}

	return org, nil
}

func getGitRepo(gitURL *url.URL) (string, error) {
	org, repo := path.Split(gitURL.Path)

	org = removeSlashes(org)
	repo = removeSlashes(repo)

	if org == "" || repo == "" {
		return "", errors.New("invalid url format, unable to find repo name: " + gitURL.Path)
	}

	return strings.TrimSuffix(repo, gitExtension), nil
}

func getAlias(url *url.URL) (string, error) {
	org, err := getGitOrg(url)
	if err != nil {
		return "", fmt.Errorf("unable to create alias: %w", err)
	}

	repo, err := getGitRepo(url)
	if err != nil {
		return "", fmt.Errorf("unable to create alias: %w", err)
	}

	return makeAlias(org, repo), nil
}

func makeAlias(org, repo string) string {
	return org + "/" + repo
}

func MakeKey(alias, subnet string) string {
	return alias + ":" + subnet
}

func splitKey(subnetKey string) (string, string, error) {
	splitSubnet := strings.Split(subnetKey, ":")
	if len(splitSubnet) != 2 {
		return "", "", fmt.Errorf("invalid subnet key: %s", subnetKey)
	}
	repo := splitSubnet[0]
	subnetName := splitSubnet[1]
	return repo, subnetName, nil
}
