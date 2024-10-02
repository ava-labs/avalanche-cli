// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/internal/testutils"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/logging"
)

const (
	version1 = "v1.17.1"
	version2 = "v1.18.1"

	avalanchegoBin = "avalanchego"
)

var (
	binary1 = []byte{0xde, 0xad, 0xbe, 0xef}
	binary2 = []byte{0xfe, 0xed, 0xc0, 0xde}
)

func setupInstallDir(require *require.Assertions) *application.Avalanche {
	rootDir, err := os.MkdirTemp(os.TempDir(), "binutils-tests")
	require.NoError(err)
	defer os.RemoveAll(rootDir)

	app := application.New()
	app.Setup(rootDir, logging.NoLog{}, &config.Config{}, prompts.NewPrompter())
	return app
}

func Test_installAvalancheGoWithVersion_Zip(t *testing.T) {
	require := testutils.SetupTest(t)
	app := setupInstallDir(require)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")

	githubDownloader := NewAvalancheGoDownloader()
	expectedDir := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version1)

	binDir, err := installBinaryWithVersion(
		app,
		version1,
		app.GetAvalanchegoBinDir(),
		avalanchegoBinPrefix,
		githubDownloader,
		mockInstaller,
	)
	require.Equal(expectedDir, binDir)
	require.NoError(err)

	// Check the installed binary
	installedBin, err := os.ReadFile(filepath.Join(binDir, avalanchegoBin))
	require.NoError(err)
	require.Equal(binary1, installedBin)
}

func Test_installAvalancheGoWithVersion_Tar(t *testing.T) {
	require := testutils.SetupTest(t)

	app := setupInstallDir(require)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "linux")

	downloader := NewAvalancheGoDownloader()

	expectedDir := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version1)

	binDir, err := installBinaryWithVersion(app, version1, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	require.Equal(expectedDir, binDir)
	require.NoError(err)

	// Check the installed binary
	installedBin, err := os.ReadFile(filepath.Join(binDir, avalanchegoBin))
	require.NoError(err)
	require.Equal(binary1, installedBin)
}

func Test_installAvalancheGoWithVersion_MultipleCoinstalls(t *testing.T) {
	require := testutils.SetupTest(t)

	zipBytes1 := testutils.CreateDummyAvagoZip(require, binary1)
	zipBytes2 := testutils.CreateDummyAvagoZip(require, binary2)
	app := setupInstallDir(require)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")

	downloader := NewAvalancheGoDownloader()
	url1, _, err := downloader.GetDownloadURL(version1, mockInstaller)
	require.NoError(err)
	url2, _, err := downloader.GetDownloadURL(version2, mockInstaller)
	require.NoError(err)
	mockInstaller.On("DownloadRelease", url1).Return(zipBytes1, nil)
	mockInstaller.On("DownloadRelease", url2).Return(zipBytes2, nil)

	expectedDir1 := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version1)
	expectedDir2 := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version2)

	binDir1, err := installBinaryWithVersion(app, version1, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	require.Equal(expectedDir1, binDir1)
	require.NoError(err)

	binDir2, err := installBinaryWithVersion(app, version2, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	require.Equal(expectedDir2, binDir2)
	require.NoError(err)

	require.NotEqual(binDir1, binDir2)

	// Check the installed binary
	installedBin1, err := os.ReadFile(filepath.Join(binDir1, avalanchegoBin))
	require.NoError(err)
	require.Equal(binary1, installedBin1)

	installedBin2, err := os.ReadFile(filepath.Join(binDir2, avalanchegoBin))
	require.NoError(err)
	require.Equal(binary2, installedBin2)
}

func Test_installSubnetEVMWithVersion(t *testing.T) {
	require := testutils.SetupTest(t)
	app := setupInstallDir(require)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")

	downloader := NewSubnetEVMDownloader()

	expectedDir := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version1)

	subDir := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version1)

	binDir, err := installBinaryWithVersion(app, version1, subDir, subnetEVMBinPrefix, downloader, mockInstaller)
	require.Equal(expectedDir, binDir)
	require.NoError(err)

	// Check the installed binary
	installedBin, err := os.ReadFile(filepath.Join(binDir, constants.SubnetEVMBin))
	require.NoError(err)
	require.Equal(binary1, installedBin)
}

func Test_installSubnetEVMWithVersion_MultipleCoinstalls(t *testing.T) {
	require := testutils.SetupTest(t)
	app := setupInstallDir(require)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("arm64", "linux")

	downloader := NewSubnetEVMDownloader()

	expectedDir1 := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version1)
	expectedDir2 := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version2)

	subDir1 := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version1)
	subDir2 := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version2)

	binDir1, err := installBinaryWithVersion(app, version1, subDir1, subnetEVMBinPrefix, downloader, mockInstaller)
	require.Equal(expectedDir1, binDir1)
	require.NoError(err)

	binDir2, err := installBinaryWithVersion(app, version2, subDir2, subnetEVMBinPrefix, downloader, mockInstaller)
	require.Equal(expectedDir2, binDir2)
	require.NoError(err)

	require.NotEqual(binDir1, binDir2)

	// Check the installed binary
	installedBin1, err := os.ReadFile(filepath.Join(binDir1, constants.SubnetEVMBin))
	require.NoError(err)
	require.Equal(binary1, installedBin1)

	installedBin2, err := os.ReadFile(filepath.Join(binDir2, constants.SubnetEVMBin))
	require.NoError(err)
	require.Equal(binary2, installedBin2)
}
