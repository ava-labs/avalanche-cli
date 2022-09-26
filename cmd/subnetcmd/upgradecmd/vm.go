// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

// avalanche subnet delete
func newVMCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "vm [subnetName]",
		Short:        "Upgrade a subnet's binary",
		Long:         "",
		RunE:         upgradeVM,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
	}
}

func upgradeVM(cmd *cobra.Command, args []string) error {
	subnetName := args[0]

	if !app.SubnetConfigExists(subnetName) {
		return errors.New("subnet does not exist")
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return fmt.Errorf("unable to load sidecar: %w", err)
	}

	vmType := sc.VM
	if vmType == models.SubnetEvm || vmType == models.SpacesVM {
		return selectUpdateOption(subnetName, vmType, sc)
	}

	// Must be a custom update
	return updateToCustomBin(subnetName, vmType, sc)
}

func selectUpdateOption(subnetName string, vmType models.VMType, sc models.Sidecar) error {
	latestVersionUpdate := "Update to latest version"
	specificVersionUpdate := "Update to a specific version"
	customBinaryUpdate := "Update to a custom binary"

	updateOptions := []string{latestVersionUpdate, specificVersionUpdate, customBinaryUpdate}

	updatePrompt := "How would you like to update your subnet's virtual machine"
	updateDecision, err := app.Prompt.CaptureList(updatePrompt, updateOptions)
	if err != nil {
		return err
	}

	switch updateDecision {
	case latestVersionUpdate:
		return updateToLatestVersion(subnetName, vmType, sc)
	case specificVersionUpdate:
		return updateToSpecificVersion(subnetName, vmType, sc)
	case customBinaryUpdate:
		return updateToCustomBin(subnetName, vmType, sc)
	default:
		return errors.New("invalid option")
	}
}

func updateToLatestVersion(subnetName string, vmType models.VMType, sc models.Sidecar) error {
	fmt.Println("Updating to latest version")
	// pull in current version
	currentVersion := sc.VMVersion

	// check latest version

	// check if current version equals latest
	if currentVersion == "latest" {
		ux.Logger.PrintToUser("VM already up-to-date")
	}

	// install latest version

	// update sidecar
	return nil
}

func updateToSpecificVersion(subnetName string, vmType models.VMType, sc models.Sidecar) error {
	fmt.Println("Updating to specific version")
	// pull in current version

	// check if current version equals chosen version

	// install specific version

	// update sidecar
	return nil
}

func updateToCustomBin(subnetName string, vmType models.VMType, sc models.Sidecar) error {
	fmt.Println("Updating to custom binary")
	// get path

	// install update

	// update sidecar
	return nil
}
