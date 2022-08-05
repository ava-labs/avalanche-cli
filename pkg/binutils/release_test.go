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
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	version1 = "v1.17.1"
	version2 = "v1.18.1"

	avalanchegoBin = "avalanchego"
	subnetEVMBin   = "subnet-evm"
)

var (
	binary1 = []byte{0xde, 0xad, 0xbe, 0xef}
	binary2 = []byte{0xfe, 0xed, 0xc0, 0xde}
)

func setupInstallDir(assert *assert.Assertions) *application.Avalanche {
	rootDir, err := os.MkdirTemp(os.TempDir(), "binutils-tests")
	assert.NoError(err)
	defer os.RemoveAll(rootDir)

	app := application.New()
	app.Setup(rootDir, logging.NoLog{}, &config.Config{}, prompts.NewPrompter())
	return app
}

func Test_installAvalancheGoWithVersion_Zip(t *testing.T) {
	assert := testutils.SetupTest(t)

	zipBytes := testutils.CreateDummyAvagoZip(assert, binary1)
	app := setupInstallDir(assert)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")

	downloader := NewAvagoDownloader()

	mockInstaller.On("DownloadRelease", mock.Anything).Return(zipBytes, nil)

	expectedDir := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version1)

	binDir, err := installBinaryWithVersion(app, version1, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir, binDir)
	assert.NoError(err)

	// Check the installed binary
	installedBin, err := os.ReadFile(filepath.Join(binDir, avalanchegoBin))
	assert.NoError(err)
	assert.Equal(binary1, installedBin)
}

func Test_installAvalancheGoWithVersion_Tar(t *testing.T) {
	assert := testutils.SetupTest(t)

	tarBytes := testutils.CreateDummyAvagoTar(assert, binary1, version1)

	app := setupInstallDir(assert)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "linux")

	downloader := NewAvagoDownloader()

	mockInstaller.On("DownloadRelease", mock.Anything).Return(tarBytes, nil)

	expectedDir := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version1)

	binDir, err := installBinaryWithVersion(app, version1, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir, binDir)
	assert.NoError(err)

	// Check the installed binary
	installedBin, err := os.ReadFile(filepath.Join(binDir, avalanchegoBin))
	assert.NoError(err)
	assert.Equal(binary1, installedBin)
}

func Test_installAvalancheGoWithVersion_MultipleCoinstalls(t *testing.T) {
	assert := testutils.SetupTest(t)

	zipBytes1 := testutils.CreateDummyAvagoZip(assert, binary1)
	zipBytes2 := testutils.CreateDummyAvagoZip(assert, binary2)
	app := setupInstallDir(assert)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")

	downloader := NewAvagoDownloader()
	url1, _, err := downloader.GetDownloadURL(version1, mockInstaller)
	assert.NoError(err)
	url2, _, err := downloader.GetDownloadURL(version2, mockInstaller)
	assert.NoError(err)
	mockInstaller.On("DownloadRelease", url1).Return(zipBytes1, nil)
	mockInstaller.On("DownloadRelease", url2).Return(zipBytes2, nil)

	expectedDir1 := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version1)
	expectedDir2 := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version2)

	binDir1, err := installBinaryWithVersion(app, version1, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir1, binDir1)
	assert.NoError(err)

	binDir2, err := installBinaryWithVersion(app, version2, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir2, binDir2)
	assert.NoError(err)

	assert.NotEqual(binDir1, binDir2)

	// Check the installed binary
	installedBin1, err := os.ReadFile(filepath.Join(binDir1, avalanchegoBin))
	assert.NoError(err)
	assert.Equal(binary1, installedBin1)

	installedBin2, err := os.ReadFile(filepath.Join(binDir2, avalanchegoBin))
	assert.NoError(err)
	assert.Equal(binary2, installedBin2)
}

func Test_installSubnetEVMWithVersion(t *testing.T) {
	assert := testutils.SetupTest(t)

	tarBytes := testutils.CreateDummySubnetEVMTar(assert, binary1)
	app := setupInstallDir(assert)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")

	downloader := NewSubnetEVMDownloader()

	mockInstaller.On("DownloadRelease", mock.Anything).Return(tarBytes, nil)

	expectedDir := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version1)

	subDir := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version1)

	binDir, err := installBinaryWithVersion(app, version1, subDir, subnetEVMBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir, binDir)
	assert.NoError(err)

	// Check the installed binary
	installedBin, err := os.ReadFile(filepath.Join(binDir, subnetEVMBin))
	assert.NoError(err)
	assert.Equal(binary1, installedBin)
}

func Test_installSubnetEVMWithVersion_MultipleCoinstalls(t *testing.T) {
	assert := testutils.SetupTest(t)

	tarBytes1 := testutils.CreateDummySubnetEVMTar(assert, binary1)
	tarBytes2 := testutils.CreateDummySubnetEVMTar(assert, binary2)
	app := setupInstallDir(assert)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("arm64", "linux")

	downloader := NewSubnetEVMDownloader()
	url1, _, err := downloader.GetDownloadURL(version1, mockInstaller)
	assert.NoError(err)
	url2, _, err := downloader.GetDownloadURL(version2, mockInstaller)
	assert.NoError(err)
	mockInstaller.On("DownloadRelease", url1).Return(tarBytes1, nil)
	mockInstaller.On("DownloadRelease", url2).Return(tarBytes2, nil)

	expectedDir1 := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version1)
	expectedDir2 := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version2)

	subDir1 := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version1)
	subDir2 := filepath.Join(app.GetSubnetEVMBinDir(), subnetEVMBinPrefix+version2)

	binDir1, err := installBinaryWithVersion(app, version1, subDir1, subnetEVMBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir1, binDir1)
	assert.NoError(err)

	binDir2, err := installBinaryWithVersion(app, version2, subDir2, subnetEVMBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir2, binDir2)
	assert.NoError(err)

	assert.NotEqual(binDir1, binDir2)

	// Check the installed binary
	installedBin1, err := os.ReadFile(filepath.Join(binDir1, subnetEVMBin))
	assert.NoError(err)
	assert.Equal(binary1, installedBin1)

	installedBin2, err := os.ReadFile(filepath.Join(binDir2, subnetEVMBin))
	assert.NoError(err)
	assert.Equal(binary2, installedBin2)
}
