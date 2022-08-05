// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testutils

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

	"github.com/stretchr/testify/assert"
)

const (
	avalanchegoBin = "avalanchego"
	pluginDirName  = "plugins"
	evmBin         = "evm"
	buildDirName   = "build"
	subnetEVMBin   = "subnet-evm"
	readme         = "README.md"
	license        = "LICENSE"

	avalanchegoBinPrefix = "avalanchego-"

	avagoTar     = "/tmp/avago.tar.gz"
	avagoZip     = "/tmp/avago.zip"
	subnetEVMTar = "/tmp/subevm.tar.gz"
)

var (
	evmBinary       = []byte{0x00, 0xe1, 0x40, 0x00}
	readmeContents  = []byte("README")
	licenseContents = []byte("LICENSE")
)

func verifyAvagoTarContents(assert *assert.Assertions, tarBytes []byte, version string) {
	topDir := avalanchegoBinPrefix + version
	bin := filepath.Join(topDir, avalanchegoBin)
	plugins := filepath.Join(topDir, pluginDirName)
	evm := filepath.Join(plugins, evmBin)

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
			// we don't need to check the top dir, it is implied through other checks
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

func CreateDummyAvagoZip(assert *assert.Assertions, binary []byte) []byte {
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
	CreateZip(assert, topDir, avagoZip)
	defer os.Remove(avagoZip)

	verifyAvagoZipContents(assert, avagoZip)

	zipBytes, err := os.ReadFile(avagoZip)
	assert.NoError(err)
	return zipBytes
}

func CreateDummyAvagoTar(assert *assert.Assertions, binary []byte, version string) []byte {
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
	CreateTarGz(assert, topDir, avagoTar, true)
	defer os.Remove(avagoTar)
	tarBytes, err := os.ReadFile(avagoTar)
	assert.NoError(err)
	verifyAvagoTarContents(assert, tarBytes, version)
	return tarBytes
}

func CreateDummySubnetEVMTar(assert *assert.Assertions, binary []byte) []byte {
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
	CreateTarGz(assert, sourceDir, subnetEVMTar, false)
	defer os.Remove(subnetEVMTar)
	tarBytes, err := os.ReadFile(subnetEVMTar)
	assert.NoError(err)
	verifySubnetEVMTarContents(assert, tarBytes)
	return tarBytes
}
