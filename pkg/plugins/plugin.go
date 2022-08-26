// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package plugins

import (
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-network-runner/utils"
)

const (
	subnetEvm = "SubnetEVM"
	customVM  = "Custom"
)

// Downloads the subnet's VM (if necessary) and copies it into the plugin directory
func CreatePlugin(app *application.Avalanche, subnetName string, pluginDir string) (string, error) {
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return "", fmt.Errorf("failed to load sidecar: %w", err)
	}

	var vmSourcePath string
	var vmDestPath string

	if sc.ImportedFromAPM {
		vmSourcePath = binutils.SetupAPMBin(app, sc.ImportedVMID)
		vmDestPath = filepath.Join(pluginDir, sc.ImportedVMID)
	} else {
		// Not imported
		chainVMID, err := utils.VMID(subnetName)
		if err != nil {
			return "", fmt.Errorf("failed to create VM ID from %s: %w", subnetName, err)
		}

		switch sc.VM {
		case subnetEvm:
			vmSourcePath, err = binutils.SetupSubnetEVM(app, sc.VMVersion)
			if err != nil {
				return "", fmt.Errorf("failed to install subnet-evm: %w", err)
			}
		case customVM:
			vmSourcePath = binutils.SetupCustomBin(app, subnetName)
		default:
			return "", fmt.Errorf("unknown vm: %s", sc.VM)
		}
		vmDestPath = filepath.Join(pluginDir, chainVMID.String())
	}

	return vmDestPath, binutils.CopyFile(vmSourcePath, vmDestPath)
}
