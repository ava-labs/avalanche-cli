package binutils

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

var (
	binary1 = []byte{0xde, 0xad, 0xbe, 0xef}
	binary2 = []byte{0xfe, 0xed, 0xc0, 0xde}
)

func setupTest(t *testing.T) *assert.Assertions {
	// use io.Discard to not print anything
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	return assert.New(t)
}

func verifyTarContents(assert *assert.Assertions, tarBytes []byte, version string) {
	topDir := avalanchegoBinPrefix + version
	bin := filepath.Join(topDir, "avalanchego")
	plugins := filepath.Join(topDir, "plugins")
	evm := filepath.Join(plugins, "evm")

	topDirExists := false
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
			topDirExists = true
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
	assert.True(topDirExists)
	assert.True(binExists)
	assert.True(pluginsExists)
	assert.True(evmExists)
}

func createDummyAvagoZip(assert *assert.Assertions, binary []byte) []byte {
	sourceDir, err := os.MkdirTemp(os.TempDir(), "binutils-source")
	assert.NoError(err)
	defer os.RemoveAll(sourceDir)
	zipDir := filepath.Join(sourceDir, "build")
	err = os.Mkdir(zipDir, 0o700)
	assert.NoError(err)
	binFilename := "avalanchego"
	binPath := filepath.Join(zipDir, binFilename)
	err = os.WriteFile(binPath, binary, 0o600)
	assert.NoError(err)

	// Put into zip
	zipFile := "/tmp/avago.zip"
	createZip(assert, zipDir, zipFile)
	zipBytes, err := os.ReadFile(zipFile)
	assert.NoError(err)
	return zipBytes
}

func createDummyAvagoTar(assert *assert.Assertions, binary []byte, version string) []byte {
	sourceDir, err := os.MkdirTemp(os.TempDir(), "binutils-source")
	assert.NoError(err)
	// defer os.RemoveAll(sourceDir)
	tarDir := filepath.Join(sourceDir, "build")
	err = os.Mkdir(tarDir, 0o700)
	assert.NoError(err)
	binFilename := "avalanchego"
	binPath := filepath.Join(tarDir, binFilename)
	err = os.WriteFile(binPath, binary, 0o600)
	assert.NoError(err)

	// Put into zip
	tarFile := "/tmp/avago.tar.gz"
	createTarGz(assert, tarDir, tarFile)
	tarBytes, err := os.ReadFile(tarFile)
	assert.NoError(err)
	verifyTarContents(assert, tarBytes, version)
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

	version := "v1.17.1"

	zipBytes := createDummyAvagoZip(assert, binary1)
	app := setupInstallDir(assert)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")

	downloader := NewAvagoDownloader()

	mockInstaller.On("DownloadRelease", mock.Anything).Return(zipBytes, nil)

	expectedDir := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version)

	binDir, err := installBinaryWithVersion(app, version, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir, binDir)
	assert.NoError(err)

	// Check the installed binary
	installedBin, err := os.ReadFile(filepath.Join(binDir, "avalanchego"))
	assert.NoError(err)
	assert.Equal(binary1, installedBin)
}

func Test_installAvalancheGoWithVersion_Tar(t *testing.T) {
	assert := setupTest(t)

	version := "v1.17.1"

	tarBytes := createDummyAvagoTar(assert, binary1, version)

	app := setupInstallDir(assert)

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "linux")

	downloader := NewAvagoDownloader()

	mockInstaller.On("DownloadRelease", mock.Anything).Return(tarBytes, nil)

	expectedDir := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version)

	binDir, err := installBinaryWithVersion(app, version, app.GetAvalanchegoBinDir(), avalanchegoBinPrefix, downloader, mockInstaller)
	assert.Equal(expectedDir, binDir)
	assert.NoError(err)

	// Check the installed binary
	installedBin, err := os.ReadFile(filepath.Join(binDir, "avalanchego"))
	assert.NoError(err)
	assert.Equal(binary1, installedBin)
}

func Test_installAvalancheGoWithVersion_MultipleCoinstalls(t *testing.T) {
	assert := setupTest(t)

	version1 := "v1.17.1"
	version2 := "v1.18.1"

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
	installedBin1, err := os.ReadFile(filepath.Join(binDir1, "avalanchego"))
	assert.NoError(err)
	assert.Equal(binary1, installedBin1)

	installedBin2, err := os.ReadFile(filepath.Join(binDir2, "avalanchego"))
	assert.NoError(err)
	assert.Equal(binary2, installedBin2)
}
