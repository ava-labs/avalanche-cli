// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func CreateCustomSubnetConfig(
	app *application.Avalanche,
	subnetName string,
	genesisPath string,
	useRepo bool,
	customVMRepoURL string,
	customVMBranch string,
	customVMBuildScript string,
	vmPath string,
) ([]byte, *models.Sidecar, error) {
	ux.Logger.PrintToUser("creating custom VM subnet %s", subnetName)

	genesisBytes, err := loadCustomGenesis(app, genesisPath)
	if err != nil {
		return nil, &models.Sidecar{}, err
	}

	sc := &models.Sidecar{
		Name:      subnetName,
		VM:        models.CustomVM,
		VMVersion: "",
		Subnet:    subnetName,
		TokenName: "",
	}

	if customVMRepoURL != "" || customVMBranch != "" || customVMBuildScript != "" {
		useRepo = true
	}
	if vmPath == "" && !useRepo {
		githubOption := "Download and build from a git repository (recommended for cloud deployments)"
		localOption := "I already have a VM binary (local network deployments only)"
		options := []string{githubOption, localOption}
		option, err := app.Prompt.CaptureList("How do you want to set up the VM binary?", options)
		if err != nil {
			return nil, &models.Sidecar{}, err
		}
		if option == githubOption {
			useRepo = true
		} else {
			vmPath, err = app.Prompt.CaptureExistingFilepath("Enter path to VM binary")
			if err != nil {
				return nil, &models.Sidecar{}, err
			}
		}
	}
	if useRepo {
		if err := SetCustomVMSourceCodeFields(app, sc, customVMRepoURL, customVMBranch, customVMBuildScript); err != nil {
			return nil, &models.Sidecar{}, err
		}
		if err := BuildCustomVM(app, sc); err != nil {
			return nil, &models.Sidecar{}, err
		}
		vmPath = app.GetCustomVMPath(subnetName)
	} else {
		if err := app.CopyVMBinary(vmPath, subnetName); err != nil {
			return nil, &models.Sidecar{}, err
		}
	}

	rpcVersion, err := GetVMBinaryProtocolVersion(vmPath)
	if err != nil {
		return nil, &models.Sidecar{}, fmt.Errorf("unable to get RPC version: %w", err)
	}

	sc.RPCVersion = rpcVersion

	return genesisBytes, sc, nil
}

func loadCustomGenesis(app *application.Avalanche, genesisPath string) ([]byte, error) {
	var err error
	if genesisPath == "" {
		genesisPath, err = app.Prompt.CaptureExistingFilepath("Enter path to custom genesis")
		if err != nil {
			return nil, err
		}
	}

	genesisBytes, err := os.ReadFile(genesisPath)
	return genesisBytes, err
}

func SetCustomVMSourceCodeFields(app *application.Avalanche, sc *models.Sidecar, customVMRepoURL string, customVMBranch string, customVMBuildScript string) error {
	var err error
	if customVMRepoURL != "" {
		ux.Logger.PrintToUser("Checking source code repository URL %s", customVMRepoURL)
		if err := prompts.ValidateURL(customVMRepoURL); err != nil {
			ux.Logger.PrintToUser("Invalid repository url %s: %s", customVMRepoURL, err)
			customVMRepoURL = ""
		}
	}
	if customVMRepoURL == "" {
		customVMRepoURL, err = app.Prompt.CaptureURL("Source code repository URL")
		if err != nil {
			return err
		}
	}
	if customVMBranch != "" {
		ux.Logger.PrintToUser("Checking branch %s", customVMBranch)
		if err := prompts.ValidateRepoBranch(customVMRepoURL, customVMBranch); err != nil {
			ux.Logger.PrintToUser("Invalid repository branch %s: %s", customVMBranch, err)
			customVMBranch = ""
		}
	}
	if customVMBranch == "" {
		customVMBranch, err = app.Prompt.CaptureRepoBranch("Branch", customVMRepoURL)
		if err != nil {
			return err
		}
	}
	if customVMBuildScript != "" {
		ux.Logger.PrintToUser("Checking build script %s", customVMBuildScript)
		if err := prompts.ValidateRepoFile(customVMRepoURL, customVMBranch, customVMBuildScript); err != nil {
			ux.Logger.PrintToUser("Invalid repository build script %s: %s", customVMBuildScript, err)
			customVMBuildScript = ""
		}
	}
	if customVMBuildScript == "" {
		customVMBuildScript, err = app.Prompt.CaptureRepoFile("Build script", customVMRepoURL, customVMBranch)
		if err != nil {
			return err
		}
	}
	sc.CustomVMRepoURL = customVMRepoURL
	sc.CustomVMBranch = customVMBranch
	sc.CustomVMBuildScript = customVMBuildScript
	return nil
}

func checkGitIsInstalled() error {
	if err := exec.Command("git").Run(); errors.Is(err, exec.ErrNotFound) {
		ux.Logger.PrintToUser("Git tool is not available. It is a necessary dependency for CLI to import a custom VM.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please follow install instructions at https://git-scm.com/book/en/v2/Getting-Started-Installing-Git and try again")
		ux.Logger.PrintToUser("")
		return err
	}
	return nil
}

func BuildCustomVM(
	app *application.Avalanche,
	sc *models.Sidecar,
) error {
	if err := checkGitIsInstalled(); err != nil {
		return err
	}

	// create repo dir
	reposDir := app.GetReposDir()
	repoDir := filepath.Join(reposDir, sc.Name)
	_ = os.RemoveAll(repoDir)
	if err := os.MkdirAll(repoDir, constants.DefaultPerms755); err != nil {
		return err
	}

	// get branch from repo
	cmd := exec.Command("git", "clone", "--single-branch", "-b", sc.CustomVMBranch, sc.CustomVMRepoURL, repoDir) //nolint:gosec
	utils.SetupRealtimeCLIOutput(cmd, true, true)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("could not clone git branch %s of repository %s: %w", sc.CustomVMBranch, sc.CustomVMRepoURL, err)
	}

	vmPath := app.GetCustomVMPath(sc.Name)
	_ = os.RemoveAll(vmPath)

	// build
	cmd = exec.Command(sc.CustomVMBuildScript, vmPath) //nolint:gosec
	cmd.Dir = repoDir
	utils.SetupRealtimeCLIOutput(cmd, true, true)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error building custom vm binary using script %s on repo %s: %w", sc.CustomVMBuildScript, sc.CustomVMRepoURL, err)
	}
	if !utils.FileExists(vmPath) {
		return fmt.Errorf("custom VM binary %s not found. Expected build script to create it as specified on the first script argument", vmPath)
	}
	if !utils.IsExecutable(vmPath) {
		return fmt.Errorf("custom VM binary %s not executable. Expected build script to create an executable file", vmPath)
	}
	return nil
}
