// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/apmintegration"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	overwriteImport bool
	repoOrURL       string
	subnetAlias     string
	branch          string
)

// avalanche subnet import
func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "import [subnetPath]",
		Short:        "Import an existing subnet config",
		RunE:         importSubnet,
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `The subnet import command will import a subnet configuration from a file or a git repository.

To import from a file, you can optionally provide the filepath as a command line argument.
Alternatively, running the command without any arguments will trigger an interactive wizard.
To import from a repo, go through the wizard. By default, an imported subnet will not
overwrite an existing subnet with the same name. To allow overwrites, provide the --force
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

func importSubnet(cmd *cobra.Command, args []string) error {
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

	err = app.WriteGenesisFile(subnetName, importable.Genesis)
	if err != nil {
		return err
	}

	err = app.CreateSidecar(&importable.Sidecar)
	if err != nil {
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

	version := fmt.Sprintf("%d.%d.%d", vmDescr.Version.Major, vmDescr.Version.Minor, vmDescr.Version.Patch)

	sidecar := models.Sidecar{
		Name:            subnetDescr.Alias,
		VM:              vmType,
		VMVersion:       version,
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
