package binutils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

// getVMBinary downloads the binary from the binary server URL
// func (d *pluginBinaryDownloader) DownloadVM(name string, vmID string, pluginDir, binDir string) error {
func SetupSubnetEVM(app *application.Avalanche, subnetEVMVersion string) (string, error) {
	// Check if already installed
	binDir := app.GetSubnetEVMBinDir()
	binChecker := NewBinaryChecker()
	exists, subnetEVMDir, err := binChecker.ExistsWithVersion(binDir, avalanchegoBinPrefix, subnetEVMVersion)
	if err != nil {
		return "", fmt.Errorf("failed trying to locate subnet-evm binary: %s", binDir)
	}
	if exists {
		app.Log.Debug("subnet-evm " + subnetEVMVersion + " found. Skipping installation")
		return subnetEVMDir, nil
	}

	// if custom, copy binary from app vm location
	sidecar, err := d.app.LoadSidecar(name)
	if err != nil {
		return err
	}
	if sidecar.VM == models.CustomVM {
		from := d.app.GetCustomVMPath(name)
		if err := copyFile(from, binaryPath); err != nil {
			return fmt.Errorf("failed copying custom vm to plugin dir: %w", err)
		}
		return nil
	}

	// not custom, download or copy subnet evm
	exists, subnetEVMDir, err := binChecker.ExistsWithLatestVersion(binDir, subnetEVMName+"-v")
	if err != nil {
		return fmt.Errorf("failed trying to locate plugin binary: %s", binDir)
	}
	if exists {
		d.app.Log.Debug("local plugin binary found. skipping installation")
	} else {
		ux.Logger.PrintToUser("VM binary does not exist locally, starting download...")

		cancel := make(chan struct{})
		go ux.PrintWait(cancel)

		// TODO: we are hardcoding the release version
		// until we have a better binary, dependency and version management
		// as per https://github.com/ava-labs/avalanche-cli/pull/17#discussion_r887164924
		version := constants.SubnetEVMReleaseVersion
		/*
			version, err := GetLatestReleaseVersion(constants.SubnetEVMReleaseURL)
			if err != nil {
				return fmt.Errorf("failed to get latest subnet-evm release version: %w", err)
			}
		*/

		subnetEVMDir, err = DownloadReleaseVersion(d.app.Log, subnetEVMName, version, binDir)
		if err != nil {
			return fmt.Errorf("failed downloading subnet-evm version: %w", err)
		}
		close(cancel)
		fmt.Println()
	}

	evmPath := filepath.Join(subnetEVMDir, subnetEVMName)

	if err := copyFile(evmPath, binaryPath); err != nil {
		return fmt.Errorf("failed copying subnet-evm to plugin dir: %w", err)
	}

	return nil
}

func installSubnetEVMWithVersion(app *application.Avalanche, subnetEVMVersion string, installer Installer) (string, error) {
	ux.Logger.PrintToUser("Installing subnet-evm " + subnetEVMVersion + "...")

	subnetEVMURL, ext, err := getSubnetEVMURL(subnetEVMVersion, installer)
	if err != nil {
		return "", fmt.Errorf("unable to determine avalanchego install URL: %d", err)
	}

	app.Log.Debug("starting download from %s...", subnetEVMURL)
	archive, err := installer.DownloadRelease(subnetEVMURL)
	if err != nil {
		return "", fmt.Errorf("unable to download subnet-evm: %d", err)
	}

	app.Log.Debug("download successful. installing archive...")
	binDir := app.GetSubnetEVMBinDir()
	if err := InstallArchive(ext, archive, binDir); err != nil {
		return "", err
	}
	subnetEVMSubDir := "subnet-evm-" + subnetEVMVersion
	if ext == zipExtension {
		// zip contains a build subdir instead of the subnetEVMSubDir expected from tar.gz
		if err := os.Rename(filepath.Join(binDir, "build"), filepath.Join(binDir, subnetEVMSubDir)); err != nil {
			return "", err
		}
	}
	ux.Logger.PrintToUser("Subnet-EVM installation successful")
	return filepath.Join(binDir, subnetEVMSubDir), nil
}

func getSubnetEVMURL(subnetEVMVersion string, installer Installer) (string, string, error) {
	// NOTE: if any of the underlying URLs change (github changes, release file names, etc.) this fails
	goarch, goos := installer.GetArch()

	var subnetEVMURL string
	var ext = "tar.gz"

	switch goos {
	case "linux":
		subnetEVMURL = fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/%s/%s_%s_linux_%s.tar.gz",
			constants.AvaLabsOrg,
			constants.SubnetEVMRepoName,
			subnetEVMVersion,
			constants.SubnetEVMRepoName,
			subnetEVMVersion[1:], // WARN subnet-evm isn't consistent in its release naming, it's omitting the v in the file name...
			goarch,
		)
	case "darwin":
		subnetEVMURL = fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/%s/%s_%s_darwin_%s.tar.gz",
			constants.AvaLabsOrg,
			constants.SubnetEVMRepoName,
			subnetEVMVersion,
			constants.SubnetEVMRepoName,
			subnetEVMVersion[1:],
			goarch,
		)
	default:
		return "", "", fmt.Errorf("OS not supported: %s", goos)
	}

	return subnetEVMURL, ext, nil
}
