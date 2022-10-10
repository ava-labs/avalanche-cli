// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package plugins

import (
	"fmt"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-network-runner/utils"
)

func SanitizePath(path string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	homeDir := usr.HomeDir
	if path == "~" {
		// In case of "~", which won't be caught by the "else if"
		path = homeDir
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, path[2:])
	}
	return path, nil
}

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
		case models.SubnetEvm:
			vmSourcePath, err = binutils.SetupSubnetEVM(app, sc.VMVersion)
			if err != nil {
				return "", fmt.Errorf("failed to install subnet-evm: %w", err)
			}
		case models.SpacesVM:
			vmSourcePath, err = binutils.SetupSpacesVM(app, sc.VMVersion)
			if err != nil {
				return "", fmt.Errorf("failed to install spaces-vm: %w", err)
			}
		case models.CustomVM:
			vmSourcePath = binutils.SetupCustomBin(app, subnetName)
		default:
			return "", fmt.Errorf("unknown vm: %s", sc.VM)
		}
		vmDestPath = filepath.Join(pluginDir, chainVMID.String())
	}

	return vmDestPath, binutils.CopyFile(vmSourcePath, vmDestPath)
}

// Downloads the target VM (if necessary) and copies it into the plugin directory
func CreatePluginFromVersion(
	app *application.Avalanche,
	subnetName string,
	vm models.VMType,
	version string,
	vmid string,
	pluginDir string,
) (string, error) {
	var vmSourcePath string
	var vmDestPath string
	var err error

	switch vm {
	case models.SubnetEvm:
		vmSourcePath, err = binutils.SetupSubnetEVM(app, version)
		if err != nil {
			return "", fmt.Errorf("failed to install subnet-evm: %w", err)
		}
	case models.SpacesVM:
		vmSourcePath, err = binutils.SetupSpacesVM(app, version)
		if err != nil {
			return "", fmt.Errorf("failed to install spaces-vm: %w", err)
		}
	case models.CustomVM:
		vmSourcePath = binutils.SetupCustomBin(app, subnetName)
	default:
		return "", fmt.Errorf("unknown vm: %s", vm)
	}
	vmDestPath = filepath.Join(pluginDir, vmid)

	return vmDestPath, binutils.CopyFile(vmSourcePath, vmDestPath)
}
