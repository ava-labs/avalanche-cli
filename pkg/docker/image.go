// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package docker

import (
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

// PullDockerImage pulls a docker image on a remote host.
func PullDockerImage(host *models.Host, image string) error {
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
	return utils.SplitSeparatedBytesToString(output, string('\n'))
}

// BuildDockerImage builds a docker image on a remote host.
func BuildDockerImage(host *models.Host, image string, path string, dockerfile string) error {
	if dockerfileFound, err := host.FileExists(filepath.Join(path, dockerfile)); err != nil || !dockerfileFound {
		ux.Logger.Error("Dockerfile %s not found in %s", dockerfile, path)
		return fmt.Errorf("Dockerfile %s not found in %s", dockerfile, path)
	}
	_, err := host.Command(fmt.Sprintf("cd %s && docker build -t %s -f %s .", path, image, dockerfile), nil, constants.SSHLongRunningScriptTimeout)
	return err
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
	// clone the repo
	if _, err := host.Command(fmt.Sprintf("git clone %s %s", gitRepo, tmpDir), nil, constants.SSHLongRunningScriptTimeout); err != nil {
		return err
	}
	// checkout the commit
	if _, err := host.Command(fmt.Sprintf("cd %s && git checkout %s", tmpDir, commit), nil, constants.SSHLongRunningScriptTimeout); err != nil {
		return err
	}
	// build the image
	if err := BuildDockerImage(host, image, tmpDir, "Dockerfile"); err != nil {
		return err
	}
	return nil
}

func PrepareDockerImageWithRepo(host *models.Host, image string, gitRepo string, commit string) error {
	localImageExists, _ := DockerLocalImageExists(host, image)
	if localImageExists {
		ux.Logger.Info("Docker image %s found on %s", image, host.NodeID)
		return nil
	} else {
		// try to pull it and if it fails build it
		if err := PullDockerImage(host, image); err != nil {
			ux.Logger.Info("Docker image %s not found on %s, building it from %s using %s commit/branch/tag", image, host.NodeID, gitRepo, commit)
			if err := BuildDockerImageFromGitRepo(host, image, gitRepo, commit); err != nil {
				return err
			}
		} else {
			ux.Logger.Info("Docker image %s successfully pulled on %s", image, host.NodeID)
		}
	}
	return nil
}
