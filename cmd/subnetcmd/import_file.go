// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/apmintegration"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/spf13/cobra"
)

var (
	overwriteImport bool
	repoOrURL       string
	subnetAlias     string
	branch          string
)

// avalanche subnet import
func newImportFileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "file [subnetPath]",
		Short:        "Import an existing subnet config",
		RunE:         importSubnet,
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `The subnet import command will import a subnet configuration from a file or a git repository.

To import from a file, you can optionally provide the path as a command-line argument.
Alternatively, running the command without any arguments triggers an interactive wizard.
To import from a repository, go through the wizard. By default, an imported Subnet doesn't 
overwrite an existing Subnet with the same name. To allow overwrites, provide the --force
flag.`,
	}
	cmd.Flags().BoolVarP(
		&overwriteImport,
		"force",
		"f",
		false,
		"overwrite the existing configuration if one exists",
	)
	cmd.Flags().StringVar(
		&repoOrURL,
		"repo",
		"",
		"the repo to import (ex: ava-labs/avalanche-plugins-core) or url to download the repo from",
	)
	cmd.Flags().StringVar(
		&branch,
		"branch",
		"",
		"the repo branch to use if downloading a new repo",
	)
	cmd.Flags().StringVar(
		&subnetAlias,
		"subnet",
		"",
		"the subnet configuration to import from the provided repo",
	)
	return cmd
}

func importSubnet(_ *cobra.Command, args []string) error {
	if len(args) == 1 {
		importPath := args[0]
		return importFromFile(importPath)
	}

	if repoOrURL == "" && branch == "" && subnetAlias == "" {
		fileOption := "File"
		apmOption := "Repository"
		typeOptions := []string{fileOption, apmOption}
		promptStr := "Would you like to import your subnet from a file or a repository?"
		result, err := app.Prompt.CaptureList(promptStr, typeOptions)
		if err != nil {
			return err
		}

		if result == fileOption {
			return importFromFile("")
		}
	}

	// Option must be APM
	return importFromAPM()
}

func importFromFile(importPath string) error {
	var err error
	if importPath == "" {
		promptStr := "Select the file to import your subnet from"
		importPath, err = app.Prompt.CaptureExistingFilepath(promptStr)
		if err != nil {
			return err
		}
	}

	importFileBytes, err := os.ReadFile(importPath)
	if err != nil {
		return err
	}

	importable := models.Exportable{}
	err = json.Unmarshal(importFileBytes, &importable)
	if err != nil {
		return err
	}

	subnetName := importable.Sidecar.Name
	if subnetName == "" {
		return errors.New("export data is malformed: missing subnet name")
	}

	if app.GenesisExists(subnetName) && !overwriteImport {
		return errors.New("subnet already exists. Use --" + forceFlag + " parameter to overwrite")
	}

	if importable.Sidecar.VM == models.CustomVM {
		if importable.Sidecar.CustomVMRepoURL == "" {
			return fmt.Errorf("repository url must be defined for custom vm import")
		}
		if importable.Sidecar.CustomVMBranch == "" {
			return fmt.Errorf("repository branch must be defined for custom vm import")
		}
		if importable.Sidecar.CustomVMBuildScript == "" {
			return fmt.Errorf("build script must be defined for custom vm import")
		}
		if err := checkGitIsInstalled(); err != nil {
			return err
		}

		reposDir := app.GetReposDir()
		repoDir := filepath.Join(reposDir, subnetName)
		_ = os.RemoveAll(repoDir)
		if err := os.MkdirAll(repoDir, constants.DefaultPerms755); err != nil {
			return err
		}

		// get branch from repo
		cmd := exec.Command("git", "clone", "--single-branch", "-b", importable.Sidecar.CustomVMBranch, importable.Sidecar.CustomVMRepoURL, repoDir) //nolint:gosec
		utils.SetupRealtimeCLIOutput(cmd, true, true)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("could not clone git branch %s of repository %s: %w", importable.Sidecar.CustomVMBranch, importable.Sidecar.CustomVMRepoURL, err)
		}
		vmPath := app.GetCustomVMPath(subnetName)
		_ = os.RemoveAll(vmPath)

		// build
		cmd = exec.Command(importable.Sidecar.CustomVMBuildScript, vmPath) //nolint:gosec
		cmd.Dir = repoDir
		utils.SetupRealtimeCLIOutput(cmd, true, true)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error building custom vm binary using script %s on repo %s: %w", importable.Sidecar.CustomVMBuildScript, importable.Sidecar.CustomVMRepoURL, err)
		}
		info, err := os.Stat(vmPath)
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("custom VM binary %s not found. Expected build script to create it as specified on the first script argument", vmPath)
		}
		if info.Mode()&0x0100 == 0 {
			return fmt.Errorf("custom VM binary %s not executable. Expected build script to create an executable file", vmPath)
		}
		rpcVersion, err := vm.GetVMBinaryProtocolVersion(vmPath)
		if err != nil {
			return fmt.Errorf("unable to get custom binary RPC version: %w", err)
		}
		if rpcVersion != importable.Sidecar.RPCVersion {
			return fmt.Errorf("RPC version mismatch between sidecar and vm binary (%d vs %d)", importable.Sidecar.RPCVersion, rpcVersion)
		}
	}

	if err := app.WriteGenesisFile(subnetName, importable.Genesis); err != nil {
		return err
	}

	if importable.NodeConfig != nil {
		if err := app.WriteAvagoNodeConfigFile(subnetName, importable.NodeConfig); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(app.GetAvagoNodeConfigPath(subnetName))
	}

	if importable.ChainConfig != nil {
		if err := app.WriteChainConfigFile(subnetName, importable.ChainConfig); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(app.GetChainConfigPath(subnetName))
	}

	if importable.SubnetConfig != nil {
		if err := app.WriteAvagoSubnetConfigFile(subnetName, importable.SubnetConfig); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(app.GetAvagoSubnetConfigPath(subnetName))
	}

	if importable.NetworkUpgrades != nil {
		if err := app.WriteNetworkUpgradesFile(subnetName, importable.NetworkUpgrades); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(app.GetUpgradeBytesFilepath(subnetName))
	}

	if err := app.CreateSidecar(&importable.Sidecar); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Subnet imported successfully")

	return nil
}

func importFromAPM() error {
	installedRepos, err := apmintegration.GetRepos(app)
	if err != nil {
		return err
	}

	var repoAlias string
	var repoURL *url.URL
	var promptStr string
	customRepo := "Download new repo"

	if repoOrURL != "" {
		for _, installedRepo := range installedRepos {
			if repoOrURL == installedRepo {
				repoAlias = installedRepo
				break
			}
		}
		if repoAlias == "" {
			repoAlias = customRepo
			repoURL, err = url.ParseRequestURI(repoOrURL)
			if err != nil {
				return fmt.Errorf("invalid url in flag: %w", err)
			}
		}
	}

	if repoAlias == "" {
		installedRepos = append(installedRepos, customRepo)

		promptStr := "What repo would you like to import from"
		repoAlias, err = app.Prompt.CaptureList(promptStr, installedRepos)
		if err != nil {
			return err
		}
	}

	if repoAlias == customRepo {
		if repoURL == nil {
			promptStr = "Enter your repo URL"
			repoURL, err = app.Prompt.CaptureGitURL(promptStr)
			if err != nil {
				return err
			}
		}

		if branch == "" {
			mainBranch := "main"
			masterBranch := "master"
			customBranch := "custom"
			branchList := []string{mainBranch, masterBranch, customBranch}
			promptStr = "What branch would you like to import from"
			branch, err = app.Prompt.CaptureList(promptStr, branchList)
			if err != nil {
				return err
			}
		}

		repoAlias, err = apmintegration.AddRepo(app, repoURL, branch)
		if err != nil {
			return err
		}

		err = apmintegration.UpdateRepos(app)
		if err != nil {
			return err
		}
	}

	subnets, err := apmintegration.GetSubnets(app, repoAlias)
	if err != nil {
		return err
	}

	var subnet string
	if subnetAlias != "" {
		for _, availableSubnet := range subnets {
			if subnetAlias == availableSubnet {
				subnet = subnetAlias
				break
			}
		}
		if subnet == "" {
			return fmt.Errorf("unable to find subnet %s", subnetAlias)
		}
	} else {
		promptStr = "Select a subnet to import"
		subnet, err = app.Prompt.CaptureList(promptStr, subnets)
		if err != nil {
			return err
		}
	}

	subnetKey := apmintegration.MakeKey(repoAlias, subnet)

	// Populate the sidecar and create a genesis
	subnetDescr, err := apmintegration.LoadSubnetFile(app, subnetKey)
	if err != nil {
		return err
	}

	var vmType models.VMType = models.CustomVM

	if len(subnetDescr.VMs) == 0 {
		return errors.New("no vms found in the given subnet")
	} else if len(subnetDescr.VMs) == 0 {
		return errors.New("multiple vm subnets not supported")
	}

	vmDescr, err := apmintegration.LoadVMFile(app, repoAlias, subnetDescr.VMs[0])
	if err != nil {
		return err
	}

	version := fmt.Sprintf("v%d.%d.%d", vmDescr.Version.Major, vmDescr.Version.Minor, vmDescr.Version.Patch)

	// this is automatically tagged as a custom VM, so we don't check the RPC
	rpcVersion := 0

	sidecar := models.Sidecar{
		Name:            subnetDescr.Alias,
		VM:              vmType,
		VMVersion:       version,
		RPCVersion:      rpcVersion,
		Subnet:          subnetDescr.Alias,
		TokenName:       constants.DefaultTokenName,
		Version:         constants.SidecarVersion,
		ImportedFromAPM: true,
		ImportedVMID:    vmDescr.ID,
	}

	ux.Logger.PrintToUser("Selected subnet, installing " + subnetKey)

	if err = apmintegration.InstallVM(app, subnetKey); err != nil {
		return err
	}

	err = app.CreateSidecar(&sidecar)
	if err != nil {
		return err
	}

	// Create an empty genesis
	return app.WriteGenesisFile(subnetDescr.Alias, []byte{})
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
