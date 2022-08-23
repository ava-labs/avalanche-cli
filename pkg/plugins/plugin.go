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
	chainVMID, err := utils.VMID(subnetName)
	if err != nil {
		return "", fmt.Errorf("failed to create VM ID from %s: %w", subnetName, err)
	}

	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return "", fmt.Errorf("failed to load sidecar: %w", err)
	}

	var vmSourcePath string
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

	vmDestPath := filepath.Join(pluginDir, chainVMID.String())

	return vmDestPath, binutils.CopyFile(vmSourcePath, vmDestPath)
}
