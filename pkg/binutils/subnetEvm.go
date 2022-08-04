package binutils

import (
	"fmt"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

func SetupSubnetEVM(app *application.Avalanche, subnetEVMVersion string) (string, error) {
	// Check if already installed
	binDir := app.GetSubnetEVMBinDir()
	subDir := filepath.Join(binDir, subnetEVMBinPrefix+subnetEVMVersion)

	installer := NewInstaller()
	downloader := NewSubnetEVMDownloader()
	vmDir, err := InstallBinary(app, subnetEVMVersion, binDir, subDir, subnetEVMBinPrefix, constants.AvaLabsOrg, constants.SubnetEVMRepoName, downloader, installer)
	fmt.Println("Installed to:", vmDir)
	return filepath.Join(vmDir, "subnet-evm"), err
}
