package binutils

import (
	"errors"
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

func setupTest(t *testing.T) *assert.Assertions {
	// use io.Discard to not print anything
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	return assert.New(t)
}

func Test_getAvalancheGoURL(t *testing.T) {
	type test struct {
		avagoVersion string
		goarch       string
		goos         string
		expectedURL  string
		expectedExt  string
		expectedErr  error
	}

	tests := []test{
		{
			avagoVersion: "v1.17.1",
			goarch:       "amd64",
			goos:         "linux",
			expectedURL:  "https://github.com/ava-labs/avalanchego/releases/download/v1.17.1/avalanchego-linux-amd64-v1.17.1.tar.gz",
			expectedExt:  tarExtension,
			expectedErr:  nil,
		},
		{
			avagoVersion: "v1.18.5",
			goarch:       "arm64",
			goos:         "darwin",
			expectedURL:  "https://github.com/ava-labs/avalanchego/releases/download/v1.18.5/avalanchego-macos-v1.18.5.zip",
			expectedExt:  zipExtension,
			expectedErr:  nil,
		},
		{
			avagoVersion: "v2.1.4",
			goarch:       "amd64",
			goos:         "windows",
			expectedURL:  "https://github.com/ava-labs/avalanchego/releases/download/v2.1.4/avalanchego-win-v2.1.4-experimental.zip",
			expectedExt:  zipExtension,
			expectedErr:  nil,
		},
		{
			avagoVersion: "v1.2.3",
			goarch:       "riscv",
			goos:         "solaris",
			expectedURL:  "",
			expectedExt:  "",
			expectedErr:  errors.New("OS not supported: solaris"),
		},
	}

	for _, tt := range tests {
		assert := assert.New(t)
		mockInstaller := &mocks.Installer{}
		mockInstaller.On("GetArch").Return(tt.goarch, tt.goos)

		url, ext, err := getAvalancheGoURL(tt.avagoVersion, mockInstaller)
		assert.Equal(tt.expectedURL, url)
		assert.Equal(tt.expectedExt, ext)
		assert.Equal(tt.expectedErr, err)
	}
}

func Test_installAvalancheGoWithVersion(t *testing.T) {
	assert := setupTest(t)

	version := "v1.17.1"
	avagoBinary := []byte{0xde, 0xad, 0xbe, 0xef}

	// create dummy binary
	sourceDir, err := os.MkdirTemp(os.TempDir(), "binutils-source")
	assert.NoError(err)
	defer os.RemoveAll(sourceDir)
	zipDir := filepath.Join(sourceDir, "build")
	err = os.Mkdir(zipDir, 0o700)
	assert.NoError(err)
	binFilename := "avalanchego"
	binPath := filepath.Join(zipDir, binFilename)
	err = os.WriteFile(binPath, avagoBinary, 0o600)
	assert.NoError(err)

	// Put into zip
	zipFile := "/tmp/avago.zip"
	createZip(assert, zipDir, zipFile)
	zipBytes, err := os.ReadFile(zipFile)
	assert.NoError(err)

	rootDir, err := os.MkdirTemp(os.TempDir(), "binutils-tests")
	assert.NoError(err)
	defer os.RemoveAll(rootDir)

	app := application.New()
	app.Setup(rootDir, logging.NoLog{}, &config.Config{}, prompts.NewPrompter())

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")
	mockInstaller.On("DownloadRelease", mock.Anything).Return(zipBytes, nil)

	expectedDir := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version)

	binDir, err := installAvalancheGoWithVersion(app, version, mockInstaller)
	assert.Equal(expectedDir, binDir)
	assert.NoError(err)
}

func Test_installAvalancheGoWithVersion_MultipleCoinstalls(t *testing.T) {
	assert := setupTest(t)

	version1 := "v1.17.1"
	version2 := "v1.18.1"
	avagoBinary := []byte{0xde, 0xad, 0xbe, 0xef}

	// create dummy binary
	sourceDir, err := os.MkdirTemp(os.TempDir(), "binutils-source")
	assert.NoError(err)
	defer os.RemoveAll(sourceDir)
	zipDir := filepath.Join(sourceDir, "build")
	err = os.Mkdir(zipDir, 0o700)
	assert.NoError(err)
	binFilename := "avalanchego"
	binPath := filepath.Join(zipDir, binFilename)
	err = os.WriteFile(binPath, avagoBinary, 0o600)
	assert.NoError(err)

	// Put into zip
	zipFile := "/tmp/avago.zip"
	createZip(assert, zipDir, zipFile)
	zipBytes, err := os.ReadFile(zipFile)
	assert.NoError(err)

	rootDir, err := os.MkdirTemp(os.TempDir(), "binutils-tests")
	assert.NoError(err)
	defer os.RemoveAll(rootDir)

	app := application.New()
	app.Setup(rootDir, logging.NoLog{}, &config.Config{}, prompts.NewPrompter())

	mockInstaller := &mocks.Installer{}
	mockInstaller.On("GetArch").Return("amd64", "darwin")
	mockInstaller.On("DownloadRelease", mock.Anything).Return(zipBytes, nil)

	expectedDir1 := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version1)
	expectedDir2 := filepath.Join(app.GetAvalanchegoBinDir(), avalanchegoBinPrefix+version2)

	binDir1, err := installAvalancheGoWithVersion(app, version1, mockInstaller)
	assert.Equal(expectedDir1, binDir1)
	assert.NoError(err)

	binDir2, err := installAvalancheGoWithVersion(app, version2, mockInstaller)
	assert.Equal(expectedDir2, binDir2)
	assert.NoError(err)

	assert.NotEqual(binDir1, binDir2)
}
