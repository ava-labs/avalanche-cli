// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

var (
	exportOutput        string
	customVMRepoURL     string
	customVMBranch      string
	customVMBuildScript string
)

// avalanche blockchain export
func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [blockchainName]",
		Short: "Export deployment details",
		Long: `The blockchain export command write the details of an existing Blockchain deploy to a file.

The command prompts for an output path. You can also provide one with
the --output flag.`,
		RunE: exportSubnet,
		Args: cobrautils.ExactArgs(1),
	}

	cmd.Flags().StringVarP(
		&exportOutput,
		"output",
		"o",
		"",
		"write the export data to the provided file path",
	)
	cmd.Flags().StringVar(&customVMRepoURL, "custom-vm-repo-url", "", "custom vm repository url")
	cmd.Flags().StringVar(&customVMBranch, "custom-vm-branch", "", "custom vm branch")
	cmd.Flags().StringVar(&customVMBuildScript, "custom-vm-build-script", "", "custom vm build-script")
	return cmd
}

func CallExportSubnet(blockchainName, exportPath string) error {
	exportOutput = exportPath
	return exportSubnet(nil, []string{blockchainName})
}

func exportSubnet(_ *cobra.Command, args []string) error {
	var err error
	if exportOutput == "" {
		pathPrompt := "Enter file path to write export data to"
		exportOutput, err = app.Prompt.CaptureString(pathPrompt)
		if err != nil {
			return err
		}
	}

	blockchainName := args[0]

	if !app.SidecarExists(blockchainName) {
		return fmt.Errorf("invalid blockchain %q", blockchainName)
	}

	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}

	if sc.VM == models.CustomVM {
		if sc.CustomVMRepoURL == "" {
			ux.Logger.PrintToUser("Custom VM source code repository, branch and build script not defined for subnet. Filling in the details now.")
			if customVMRepoURL != "" {
				ux.Logger.PrintToUser("Checking source code repository URL %s", customVMRepoURL)
				if err := prompts.ValidateURL(customVMRepoURL); err != nil {
					ux.Logger.PrintToUser("Invalid repository url %s: %s", customVMRepoURL, err)
					customVMRepoURL = ""
				}
			}
			if customVMRepoURL == "" {
				customVMRepoURL, err = app.Prompt.CaptureURL("Source code repository URL", true)
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
			if err := app.UpdateSidecar(&sc); err != nil {
				return err
			}
		}
	}

	gen, err := app.LoadRawGenesis(blockchainName)
	if err != nil {
		return err
	}

	var nodeConfig, chainConfig, subnetConfig, networkUpgrades []byte

	if app.AvagoNodeConfigExists(blockchainName) {
		nodeConfig, err = app.LoadRawAvagoNodeConfig(blockchainName)
		if err != nil {
			return err
		}
	}
	if app.ChainConfigExists(blockchainName) {
		chainConfig, err = app.LoadRawChainConfig(blockchainName)
		if err != nil {
			return err
		}
	}
	if app.AvagoSubnetConfigExists(blockchainName) {
		subnetConfig, err = app.LoadRawAvagoSubnetConfig(blockchainName)
		if err != nil {
			return err
		}
	}
	if app.NetworkUpgradeExists(blockchainName) {
		networkUpgrades, err = app.LoadRawNetworkUpgrades(blockchainName)
		if err != nil {
			return err
		}
	}

	exportData := models.Exportable{
		Sidecar:         sc,
		Genesis:         gen,
		NodeConfig:      nodeConfig,
		ChainConfig:     chainConfig,
		SubnetConfig:    subnetConfig,
		NetworkUpgrades: networkUpgrades,
	}

	exportBytes, err := json.Marshal(exportData)
	if err != nil {
		return err
	}
	return os.WriteFile(exportOutput, exportBytes, constants.WriteReadReadPerms)
}
