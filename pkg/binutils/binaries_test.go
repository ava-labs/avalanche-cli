// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/testutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/stretchr/testify/require"
)

func TestInstallZipArchive(t *testing.T) {
	require := require.New(t)

	archivePath, checkFunc := testutils.CreateTestArchivePath(t, require)

	tmpDir := os.TempDir()
	zip := filepath.Join(tmpDir, "testFile.zip")
	defer os.Remove(zip)

	testutils.CreateZip(require, archivePath, zip)

	// can't use t.TempDir here as that returns the same dir
	installDir, err := os.MkdirTemp(tmpDir, "zip-test-dir")
	require.NoError(err)
	defer os.RemoveAll(installDir)

	zipBytes, err := os.ReadFile(zip)
	require.NoError(err)

	err = installZipArchive(zipBytes, installDir)
	require.NoError(err)

	checkFunc(archivePath)
}

func TestInstallGzipArchive(t *testing.T) {
	require := require.New(t)

	archivePath, checkFunc := testutils.CreateTestArchivePath(t, require)

	tmpDir := os.TempDir()
	tgz := filepath.Join(tmpDir, "testFile.tar.gz")
	defer os.Remove(tgz)

	testutils.CreateTarGz(require, archivePath, tgz, true)

	// can't use t.TempDir here as that returns the same dir
	installDir, err := os.MkdirTemp(tmpDir, "gzip-test-dir")
	require.NoError(err)
	defer os.RemoveAll(installDir)

	tgzBytes, err := os.ReadFile(tgz)
	require.NoError(err)

	err = installTarGzArchive(tgzBytes, installDir)
	require.NoError(err)

	checkFunc(archivePath)
}

func TestExistsWithVersion(t *testing.T) {
	binPrefix := "binary-"
	binVersion := "1.4.3"

	require := require.New(t)

	installDir, err := os.MkdirTemp(os.TempDir(), "binutils-tests")
	require.NoError(err)
	defer os.RemoveAll(installDir)

	checker := NewBinaryChecker()

	exists, err := checker.ExistsWithVersion(installDir, binPrefix, binVersion)
	require.NoError(err)
	require.False(exists)

	err = os.Mkdir(filepath.Join(installDir, binPrefix+binVersion), constants.DefaultPerms755)
	require.NoError(err)

	exists, err = checker.ExistsWithVersion(installDir, binPrefix, binVersion)
	require.NoError(err)
	require.True(exists)
}

func TestExistsWithVersion_Longer(t *testing.T) {
	binPrefix := "binary-"
	desiredVersion := "1.4.3"
	actualVersion := "1.4.30"

	require := require.New(t)

	installDir, err := os.MkdirTemp(os.TempDir(), "binutils-tests")
	require.NoError(err)
	defer os.RemoveAll(installDir)

	checker := NewBinaryChecker()

	exists, err := checker.ExistsWithVersion(installDir, binPrefix, desiredVersion)
	require.NoError(err)
	require.False(exists)

	err = os.Mkdir(filepath.Join(installDir, binPrefix+actualVersion), constants.DefaultPerms755)
	require.NoError(err)

	exists, err = checker.ExistsWithVersion(installDir, binPrefix, desiredVersion)
	require.NoError(err)
	require.False(exists)
}
