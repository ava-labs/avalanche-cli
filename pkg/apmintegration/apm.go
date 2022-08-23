package apmintegration

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
)

func getGithubOrg(url string) (string, error) {
	split := strings.Split(url, "/")

	if len(split) < 3 {
		return "", errors.New("unable to find organization")
	}

	return split[len(split)-2], nil
}

func getGithubRepo(url string) (string, error) {
	base := filepath.Base(url)
	if base[len(base)-4:] != ".git" {
		return "", errors.New("unable to find repo name")
	}
	return base[:len(base)-4], nil
}

func getAlias(url string) (string, error) {
	org, err := getGithubOrg(url)
	if err != nil {
		return "", fmt.Errorf("unable to create alias: %w", err)
	}

	repo, err := getGithubRepo(url)
	if err != nil {
		return "", fmt.Errorf("unable to create alias: %w", err)
	}

	return org + "/" + repo, nil
}

func AddRepo(app *application.Avalanche, repoURL string, branch string) error {
	alias, err := getAlias(repoURL)
	if err != nil {
		return err
	}
	return app.Apm.AddRepository(alias, repoURL, branch)
}
