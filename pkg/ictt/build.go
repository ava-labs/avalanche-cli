// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ictt

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

func RepoDir(
	app *application.Avalanche,
) (string, error) {
	repoDir := filepath.Join(app.GetReposDir(), constants.ICTTDir)
	if err := os.MkdirAll(repoDir, constants.DefaultPerms755); err != nil {
		return "", err
	}
	return repoDir, nil
}

func BuildContracts(
	app *application.Avalanche,
) error {
	repoDir, err := RepoDir(app)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		forgePath,
		"build",
		"--extra-output-files",
		"bin",
	)
	cmd.Dir = filepath.Join(repoDir, "contracts")
	stdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, false)
	if err := cmd.Run(); err != nil {
		fmt.Println(stdout)
		fmt.Println(stderr)
		return fmt.Errorf("could not build contracts: %w", err)
	}
	return nil
}

func DownloadRepo(
	app *application.Avalanche,
	version string,
) error {
	if err := vm.CheckGitIsInstalled(); err != nil {
		return err
	}
	repoDir, err := RepoDir(app)
	if err != nil {
		return err
	}
	alreadyCloned, err := utils.NonEmptyDirectory(repoDir)
	if err != nil {
		return err
	}
	if !alreadyCloned {
		cmd := exec.Command(
			"git",
			"clone",
			"-b",
			constants.ICTTBranch,
			constants.ICTTURL,
			repoDir,
			"--recurse-submodules",
			"--shallow-submodules",
		)
		stdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, false)
		if err := cmd.Run(); err != nil {
			fmt.Println(stdout)
			fmt.Println(stderr)
			return fmt.Errorf("could not clone repository %s: %w", constants.ICTTURL, err)
		}
	} else {
		cmd := exec.Command("git", "checkout", constants.ICTTBranch)
		cmd.Dir = repoDir
		stdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, false)
		if err := cmd.Run(); err != nil {
			fmt.Println(stdout)
			fmt.Println(stderr)
			return fmt.Errorf("could not checkout commit/branch %s of repository %s: %w", constants.ICTTBranch, constants.ICTTURL, err)
		}
		cmd = exec.Command("git", "pull")
		cmd.Dir = repoDir
		stdout, stderr = utils.SetupRealtimeCLIOutput(cmd, false, false)
		if err := cmd.Run(); err != nil {
			fmt.Println(stdout)
			fmt.Println(stderr)
			return fmt.Errorf("could not pull repository %s: %w", constants.ICTTURL, err)
		}
	}
	if version != "" {
		cmd := exec.Command("git", "checkout", version)
		cmd.Dir = repoDir
		stdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, false)
		if err := cmd.Run(); err != nil {
			fmt.Println(stdout)
			fmt.Println(stderr)
			return fmt.Errorf("could not checkout commit/branch %s of repository %s: %w", version, constants.ICTTURL, err)
		}
		cmd = exec.Command("git", "branch", "--show-current")
		cmd.Dir = repoDir
		stdout, stderr = utils.SetupRealtimeCLIOutput(cmd, false, false)
		if err := cmd.Run(); err != nil {
			fmt.Println(stdout)
			fmt.Println(stderr)
			return fmt.Errorf("could not query current branch name: %w", err)
		}
		if stdout.String() != "" {
			cmd = exec.Command("git", "pull")
			cmd.Dir = repoDir
			stdout, stderr = utils.SetupRealtimeCLIOutput(cmd, false, false)
			if err := cmd.Run(); err != nil {
				fmt.Println(stdout)
				fmt.Println(stderr)
				return fmt.Errorf("could not pull repository %s: %w", constants.ICTTURL, err)
			}
		}
	}
	cmd := exec.Command(
		"git",
		"submodule",
		"update",
		"--init",
		"--recursive",
		"--single-branch",
	)
	cmd.Dir = repoDir
	stdout, stderr := utils.SetupRealtimeCLIOutput(cmd, false, false)
	if err := cmd.Run(); err != nil {
		fmt.Println(stdout)
		fmt.Println(stderr)
		return fmt.Errorf("could not update submodules of repository %s: %w", constants.ICTTURL, err)
	}
	return nil
}
