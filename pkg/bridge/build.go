// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridge

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
)

const (
	BridgeRepoDir = "teleporter-token-bridge"
)

func RepoDir(
	app *application.Avalanche,
) (string, error) {
	repoDir := filepath.Join(app.GetReposDir(), constants.BridgeDir)
	if err := os.MkdirAll(repoDir, constants.DefaultPerms755); err != nil {
		return "", err
	}
	return repoDir, nil
}

func DownloadRepo(
	app *application.Avalanche,
) error {
	if err := vm.CheckGitIsInstalled(); err != nil {
		return err
	}
	repoDir, err := RepoDir(app)
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repoDir
	utils.SetupRealtimeCLIOutput(cmd, true, true)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not init git directory on %s: %w", repoDir, err)
	}
	cmd = exec.Command("git", "remote", "-v")
	cmd.Dir = repoDir
	remoteStdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, true)
	if err := cmd.Run(); err != nil {
		fmt.Println(remoteStdout)
		fmt.Println(stderr)
		return fmt.Errorf("could check git remote conf on %s: %w", repoDir, err)
	}
	if remoteStdout.String() == "" {
		cmd = exec.Command("git", "remote", "add", "origin", constants.BridgeURL)
		cmd.Dir = repoDir
		utils.SetupRealtimeCLIOutput(cmd, true, true)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("could not add origin %s on git: %w", constants.BridgeURL, err)
		}
		cmd = exec.Command("git", "fetch", "--depth", "1", "origin", constants.BridgeBranch, "-q")
		cmd.Dir = repoDir
		utils.SetupRealtimeCLIOutput(cmd, true, true)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("could not fetch git branch/commit %s of repository %s: %w", constants.BridgeBranch, constants.BridgeURL, err)
		}
		cmd = exec.Command("git", "checkout", constants.BridgeBranch)
		cmd.Dir = repoDir
		stdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, false)
		if err := cmd.Run(); err != nil {
			fmt.Println(stdout)
			fmt.Println(stderr)
			return fmt.Errorf("could not checkout git branch %s of repository %s: %w", constants.BridgeBranch, constants.BridgeURL, err)
		}
	} else {
		cmd = exec.Command("git", "pull")
		cmd.Dir = repoDir
		stdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, false)
		if err := cmd.Run(); err != nil {
			fmt.Println(stdout)
			fmt.Println(stderr)
			return fmt.Errorf("could not pull git branch %s of repository %s: %w", constants.BridgeBranch, constants.BridgeURL, err)
		}
	}
	cmd = exec.Command(
		"git",
		"submodule",
		"update",
		"--init",
		"--recursive",
	)
	cmd.Dir = repoDir
	stdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, false)
	if err := cmd.Run(); err != nil {
		fmt.Println(stdout)
		fmt.Println(stderr)
		return fmt.Errorf("could not update submodules of repository %s: %w", constants.BridgeURL, err)
	}
	return nil
}
