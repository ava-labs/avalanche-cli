// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

// PullDockerImage pulls a docker image on a remote host.
func PullDockerImage(host *models.Host, image string) error {
	ux.Logger.Info("Pulling docker image %s on %s", image, host.NodeID)
	_, err := host.Command("docker pull "+image, nil, constants.SSHLongRunningScriptTimeout)
	return err
}

// DockerLocalImageExists checks if a docker image exists on a remote host.
func DockerLocalImageExists(host *models.Host, image string) (bool, error) {
	output, err := host.Command("docker images --format '{{.Repository}}:{{.Tag}}'", nil, constants.SSHLongRunningScriptTimeout)
	if err != nil {
		return false, err
	}
	for _, localImage := range parseDockerImageListOutput(output) {
		if localImage == image {
			return true, nil
		}
	}
	return false, nil
}

// parseDockerImageListOutput parses the output of a docker images command.
func parseDockerImageListOutput(output []byte) []string {
	return strings.Split(string(output), "\n")
}

func parseRemoteGoModFile(path string, host *models.Host) (string, error) {
	goMod := filepath.Join(path, "go.mod")
	// download and parse go.mod
	tmpFile, err := os.CreateTemp("", "go-mod-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	if err := host.Download(goMod, tmpFile.Name(), constants.SSHFileOpsTimeout); err != nil {
		return "", err
	}
	return utils.ReadGoVersion(tmpFile.Name())
}

// BuildDockerImage builds a docker image on a remote host.
func BuildDockerImage(host *models.Host, image string, path string, dockerfile string) error {
	goVersion, err := parseRemoteGoModFile(path, host)
	if err != nil {
		// fall back to default
		ux.Logger.Info("failed to read go version from go.mod: %s. falling back to default", err)
		goVersion = constants.BuildEnvGolangVersion
	}
	cmd := fmt.Sprintf("cd %s && docker build -q --build-arg GO_VERSION=%s -t %s -f %s .", path, goVersion, image, dockerfile)
	_, err = host.Command(cmd, nil, constants.SSHLongRunningScriptTimeout)
	return fmt.Errorf("failed building docker image:  %s %w", cmd, err)
}

// BuildDockerImageFromGitRepo builds a docker image from a git repo on a remote host.
func BuildDockerImageFromGitRepo(host *models.Host, image string, gitRepo string, commit string) error {
	if commit == "" {
		commit = "HEAD"
	}
	tmpDir, err := host.CreateTempDir()
	if err != nil {
		return err
	}
	defer func() {
		if err := host.Remove(tmpDir, true); err != nil {
			ux.Logger.Error("Error removing temporary directory %s: %s", tmpDir, err)
		}
	}()
	// clone the repo and checkout commit
	if _, err := host.Command(fmt.Sprintf("git clone %s %s && cd %s && git checkout %s && sleep 2", gitRepo, tmpDir, tmpDir, commit), nil, constants.SSHLongRunningScriptTimeout); err != nil {
		return err
	}
	// build the image
	if err := BuildDockerImage(host, image, tmpDir, "Dockerfile"); err != nil {
		return err
	}
	ux.Logger.Info("Docker image %s built from %s using %s commit/branch/tag", image, gitRepo, commit)
	return nil
}

func PrepareDockerImageWithRepo(host *models.Host, image string, gitRepo string, commit string) error {
	localImageExists, _ := DockerLocalImageExists(host, image)
	if localImageExists {
		ux.Logger.Info("Docker image %s is FOUND on %s", image, host.NodeID)
		return nil
	} else {
		ux.Logger.Info("Docker image %s not found on %s, pulling it", image, host.NodeID)
		if err := PullDockerImage(host, image); err != nil {
			ux.Logger.Info("Docker image %s not found on %s, building it from %s using %s commit/branch/tag", image, host.NodeID, gitRepo, commit)
			if err := BuildDockerImageFromGitRepo(host, image, gitRepo, commit); err != nil {
				return err
			}
			return nil
		}
	}
	ux.Logger.Info("Docker image %s is READY on %s", image, host.NodeID)
	return nil
}
