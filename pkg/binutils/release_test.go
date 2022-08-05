package binutils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	avalanchegoBin = "avalanchego"
	pluginDirName  = "plugins"
	evmBin         = "evm"
	buildDirName   = "build"
	subnetEVMBin   = "subnet-evm"
	readme         = "README.md"
	license        = "LICENSE"

	version1 = "v1.17.1"
	version2 = "v1.18.1"
)

var (
	binary1   = []byte{0xde, 0xad, 0xbe, 0xef}
	binary2   = []byte{0xfe, 0xed, 0xc0, 0xde}
	evmBinary = []byte{0x00, 0xe1, 0x40, 0x00}

	readmeContents  = []byte("README")
	licenseContents = []byte("LICENSE")
)

func setupTest(t *testing.T) *assert.Assertions {
	// use io.Discard to not print anything
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	return assert.New(t)
}

func verifyAvagoTarContents(assert *assert.Assertions, tarBytes []byte, version string) {
	topDir := avalanchegoBinPrefix + version
	bin := filepath.Join(topDir, avalanchegoBin)
	plugins := filepath.Join(topDir, pluginDirName)
	evm := filepath.Join(plugins, evmBin)

	// topDirExists := false
	binExists := false
	pluginsExists := false
	evmExists := false

	file := bytes.NewReader(tarBytes)
	gzRead, err := gzip.NewReader(file)
	assert.NoError(err)
	tarReader := tar.NewReader(gzRead)
	assert.NoError(err)
	for {
		file, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		fmt.Println("Archive contains:", file.Name)
		switch file.Name {
		case topDir:
			// topDirExists = true
			continue
		case bin:
			binExists = true
		case plugins:
			pluginsExists = true
		case evm:
			evmExists = true
		default:
			assert.FailNow("Tar has extra files")
		}
	}
	// assert.True(topDirExists)
	assert.True(binExists)
	assert.True(pluginsExists)
	assert.True(evmExists)
}

func verifySubnetEVMTarContents(assert *assert.Assertions, tarBytes []byte) {
	binExists := false
	readmeExists := false
	licenseExists := false

	file := bytes.NewReader(tarBytes)
	gzRead, err := gzip.NewReader(file)
	assert.NoError(err)
	tarReader := tar.NewReader(gzRead)
	assert.NoError(err)
	for {
		file, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(err)
		fmt.Println("Archive contains:", file.Name)
		switch file.Name {
		case subnetEVMBin:
			binExists = true
		case readme:
			readmeExists = true
		case license:
			licenseExists = true
		default:
			assert.FailNow("Tar has extra files: " + file.Name)
		}
	}
	assert.True(binExists)
	assert.True(readmeExists)
	assert.True(licenseExists)
}

func verifyAvagoZipContents(assert *assert.Assertions, zipFile string) {
	topDir := buildDirName
	bin := filepath.Join(topDir, avalanchegoBin)
	plugins := filepath.Join(topDir, pluginDirName)
	evm := filepath.Join(plugins, evmBin)

	topDirExists := false
	binExists := false
	pluginsExists := false
	evmExists := false

	reader, err := zip.OpenReader(zipFile)
	assert.NoError(err)
	defer reader.Close()
	for _, file := range reader.File {
		fmt.Println("Archive contains:", file.Name)
		// Zip directories end in "/" which is annoying for string matching
		switch strings.TrimSuffix(file.Name, "/") {
		case topDir:
			topDirExists = true
		case bin:
			binExists = true
		case plugins:
			pluginsExists = true
		case evm:
			evmExists = true
		default:
			assert.FailNow("Zip has extra files: " + file.Name)
		}
	}
	assert.True(topDirExists)
	assert.True(binExists)
	assert.True(pluginsExists)
	assert.True(evmExists)
}

func createDummyAvagoZip(assert *assert.Assertions, binary []byte) []byte {
	sourceDir, err := os.MkdirTemp(os.TempDir(), "binutils-source")
	assert.NoError(err)
	defer os.RemoveAll(sourceDir)

	topDir := filepath.Join(sourceDir, buildDirName)
	err = os.Mkdir(topDir, 0o700)
	assert.NoError(err)

	binPath := filepath.Join(topDir, avalanchegoBin)
	err = os.WriteFile(binPath, binary, 0o600)
	assert.NoError(err)

	pluginDir := filepath.Join(topDir, pluginDirName)
	err = os.Mkdir(pluginDir, 0o700)
	assert.NoError(err)

	evmBinPath := filepath.Join(pluginDir, evmBin)
	err = os.WriteFile(evmBinPath, evmBinary, 0o600)
	assert.NoError(err)

	// Put into zip
	zipFile := "/tmp/avago.zip"
	createZip(assert, topDir, zipFile)

	verifyAvagoZipContents(assert, zipFile)

	zipBytes, err := os.ReadFile(zipFile)
	assert.NoError(err)
	return zipBytes
}

func createDummyAvagoTar(assert *assert.Assertions, binary []byte, version string) []byte {
	sourceDir, err := os.MkdirTemp(os.TempDir(), "binutils-source")
	assert.NoError(err)
	defer os.RemoveAll(sourceDir)

	topDir := filepath.Join(sourceDir, avalanchegoBinPrefix+version)
	err = os.Mkdir(topDir, 0o700)
	assert.NoError(err)

	binPath := filepath.Join(topDir, avalanchegoBin)
	err = os.WriteFile(binPath, binary, 0o600)
	assert.NoError(err)

	pluginDir := filepath.Join(topDir, pluginDirName)
	err = os.Mkdir(pluginDir, 0o700)
	assert.NoError(err)

	evmBinPath := filepath.Join(pluginDir, evmBin)
	err = os.WriteFile(evmBinPath, evmBinary, 0o600)
	assert.NoError(err)

	// Put into tar
	tarFile := "/tmp/avago.tar.gz"
	createTarGz(assert, topDir, tarFile, true)
	tarBytes, err := os.ReadFile(tarFile)
	assert.NoError(err)
	verifyAvagoTarContents(assert, tarBytes, version)
	return tarBytes
}

func createDummySubnetEVMTar(assert *assert.Assertions, binary []byte) []byte {
	sourceDir, err := os.MkdirTemp(os.TempDir(), "binutils-source")
	assert.NoError(err)
	defer os.RemoveAll(sourceDir)

	binPath := filepath.Join(sourceDir, subnetEVMBin)
	err = os.WriteFile(binPath, binary, 0o600)
	assert.NoError(err)

	readmePath := filepath.Join(sourceDir, readme)
	err = os.WriteFile(readmePath, readmeContents, 0o600)
	assert.NoError(err)

	licensePath := filepath.Join(sourceDir, license)
	err = os.WriteFile(licensePath, licenseContents, 0o600)
	assert.NoError(err)

	// Put into tar
	tarFile := "/tmp/avago.tar.gz"
	createTarGz(assert, sourceDir, tarFile, false)
	tarBytes, err := os.ReadFile(tarFile)
	assert.NoError(err)
	verifySubnetEVMTarContents(assert, tarBytes)
	return tarBytes
}

func setupInstallDir(assert *assert.Assertions) *application.Avalanche {
	rootDir, err := os.MkdirTemp(os.TempDir(), "binutils-tests")
	assert.NoError(err)
	// defer os.RemoveAll(rootDir)

	app := application.New()
	app.Setup(rootDir, logging.NoLog{}, &config.Config{}, prompts.NewPrompter())
	return app
}

func Test_installAvalancheGoWithVersion_Zip(t *testing.T) {
	assert := setupTest(t)

	zipBytes := createDummyAvagoZip(assert, binary1)
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
	assert := setupTest(t)

	tarBytes := createDummyAvagoTar(assert, binary1, version1)

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
	assert := setupTest(t)

	zipBytes1 := createDummyAvagoZip(assert, binary1)
	zipBytes2 := createDummyAvagoZip(assert, binary2)
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
	assert := setupTest(t)

	tarBytes := createDummySubnetEVMTar(assert, binary1)
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
	assert := setupTest(t)

	tarBytes1 := createDummySubnetEVMTar(assert, binary1)
	tarBytes2 := createDummySubnetEVMTar(assert, binary2)
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
