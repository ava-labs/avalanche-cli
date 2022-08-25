// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/apmintegration"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var overwriteImport bool

// avalanche subnet import
func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "import [subnetPath]",
		Short:        "Import an existing subnet config",
		RunE:         importSubnet,
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `The subnet import command accepts an exported subnet config file.

By default, an imported subnet will not overwrite an existing subnet
with the same name. To allow overwrites, provide the --force flag.`,
	}
	cmd.Flags().BoolVarP(
		&overwriteImport,
		"force",
		"f",
		false,
		"overwrite the existing configuration if one exists",
	)
	return cmd
}

func importSubnet(cmd *cobra.Command, args []string) error {
	if len(args) == 1 {
		importPath := args[0]
		return importFromFile(importPath)
	}

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

	customRepo := "Download new repo"
	installedRepos = append(installedRepos, customRepo)

	promptStr := "What repo would you like to import from"
	repo, err := app.Prompt.CaptureList(promptStr, installedRepos)
	if err != nil {
		return err
	}

	if repo == customRepo {
		promptStr = "Enter your repo URL"
		repoURL, err := app.Prompt.CaptureString(promptStr)
		if err != nil {
			return err
		}

		mainBranch := "main"
		masterBranch := "master"
		customBranch := "custom"
		branchList := []string{mainBranch, masterBranch, customBranch}
		promptStr = "What branch would you like to import from"
		branch, err := app.Prompt.CaptureList(promptStr, branchList)
		if err != nil {
			return err
		}

		repo, err = apmintegration.AddRepo(app, repoURL, branch)
		if err != nil {
			return err
		}

		err = apmintegration.UpdateRepos(app)
		if err != nil {
			return err
		}
	}

	subnets, err := apmintegration.GetSubnets(app, repo)
	if err != nil {
		return err
	}

	promptStr = "Select a subnet to import"
	subnet, err := app.Prompt.CaptureList(promptStr, subnets)
	if err != nil {
		return err
	}

	subnetKey := repo + ":" + subnet
	ux.Logger.PrintToUser("Selected subnet, installing " + subnetKey)

	return apmintegration.InstallVM(app, subnetKey)
}
