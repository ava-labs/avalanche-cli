// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestInstallZipArchive(t *testing.T) {
	assert := assert.New(t)

	archivePath, checkFunc := testutils.CreateTestArchivePath(t, assert)

	tmpDir := os.TempDir()
	zip := filepath.Join(tmpDir, "testFile.zip")
	defer os.Remove(zip)

	testutils.CreateZip(assert, archivePath, zip)

	// can't use t.TempDir here as that returns the same dir
	installDir, err := os.MkdirTemp(tmpDir, "zip-test-dir")
	assert.NoError(err)
	defer os.RemoveAll(installDir)

	zipBytes, err := os.ReadFile(zip)
	assert.NoError(err)

	err = installZipArchive(zipBytes, installDir)
	assert.NoError(err)

	checkFunc(archivePath)
}

func TestInstallGzipArchive(t *testing.T) {
	assert := assert.New(t)

	archivePath, checkFunc := testutils.CreateTestArchivePath(t, assert)

	tmpDir := os.TempDir()
	tgz := filepath.Join(tmpDir, "testFile.tar.gz")
	defer os.Remove(tgz)

	testutils.CreateTarGz(assert, archivePath, tgz, true)

	// can't use t.TempDir here as that returns the same dir
	installDir, err := os.MkdirTemp(tmpDir, "gzip-test-dir")
	assert.NoError(err)
	defer os.RemoveAll(installDir)

	tgzBytes, err := os.ReadFile(tgz)
	assert.NoError(err)

	err = installTarGzArchive(tgzBytes, installDir)
	assert.NoError(err)

	checkFunc(archivePath)
}
