// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apmintegration

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

func getGitOrg(url string) (string, error) {
	split := strings.Split(url, "/")

	if len(split) < 3 {
		return "", errors.New("unable to find organization")
	}

	return split[len(split)-2], nil
}

func getGitRepo(url string) (string, error) {
	base := path.Base(url)
	if path.Ext(base) != ".git" {
		return "", errors.New("unable to find repo name")
	}
	return strings.TrimSuffix(base, path.Ext(base)), nil
}

func getAlias(url string) (string, error) {
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
