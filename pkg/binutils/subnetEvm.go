package binutils

import (
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

// getVMBinary downloads the binary from the binary server URL
// func (d *pluginBinaryDownloader) DownloadVM(name string, vmID string, pluginDir, binDir string) error {
func SetupSubnetEVM(app *application.Avalanche, subnetEVMVersion string) (string, error) {
	// Check if already installed
	binDir := app.GetSubnetEVMBinDir()
	subDir := filepath.Join(binDir, subnetEVMBinPrefix+subnetEVMVersion)
	binChecker := NewBinaryChecker()
	exists, subnetEVMDir, err := binChecker.ExistsWithVersion(binDir, avalanchegoBinPrefix, subnetEVMVersion)
	if err != nil {
		return "", fmt.Errorf("failed trying to locate subnet-evm binary: %s", binDir)
	}
	if exists {
		app.Log.Debug("subnet-evm " + subnetEVMVersion + " found. Skipping installation")
		return subnetEVMDir, nil
	}

	installer := NewInstaller()
	downloader := NewSubnetEVMDownloader()
	vmDir, err := InstallBinary(app, subnetEVMVersion, subDir, subnetEVMBinPrefix, constants.AvaLabsOrg, constants.SubnetEVMRepoName, downloader, installer)
	fmt.Println("Installed to:", vmDir)
	return filepath.Join(vmDir, "subnet-evm"), err
}
