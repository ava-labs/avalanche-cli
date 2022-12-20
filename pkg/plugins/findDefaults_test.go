// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/config"
	"github.com/stretchr/testify/require"
)

// TestFindByRunningProcess starts a process and then
// runs findByRunningProcesses to check if the correct
// arguments can be extracted from the process CommandLine
func TestFindByRunningProcess(t *testing.T) {
	testDir := t.TempDir()
	require := require.New(t)

	fakeSetEnvVar := "THIS_ENV_VAR_EXISTS"
	fakeNotSetEnvVar := "THIS_ENV_VAR_DOESNT_EXIST"
	noConfigFileDir := filepath.Join(testDir, "this-dir-has-no-config-file")

	scanDirs := []string{
		// firs indexes should succeed
		filepath.Join(testDir, "etc", "avalanchego"),
		filepath.Join(testDir, "home", ".avalanchego"),
		testDir,
		"$" + config.AvalancheGoDataDirVar,
		// following indexes should fail (don't exist)
		fakeNotSetEnvVar,
		fakeSetEnvVar,
		"/path/to/nirvana",
		noConfigFileDir,
	}

	err := os.MkdirAll(scanDirs[0], constants.DefaultPerms755)
	require.NoError(err)
	err = os.MkdirAll(scanDirs[1], constants.DefaultPerms755)
	require.NoError(err)

	os.Setenv("THIS_ENV_VAR_EXISTS", "/path/doesnt/matter")
	existingDataDir := filepath.Join(testDir, "data-dir")
	// make sure we don't accidentally overwrite a really existing env var
	origVar := os.Getenv(config.AvalancheGoDataDirVar)
	os.Setenv(config.AvalancheGoDataDirVar, existingDataDir)
	defer func() {
		os.Setenv(config.AvalancheGoDataDirVar, origVar)
		os.Setenv(fakeSetEnvVar, "")
	}()

	err = os.MkdirAll(existingDataDir, constants.DefaultPerms755)
	require.NoError(err)

	for _, d := range scanDirs[:3] {
		_, err = os.Create(filepath.Join(d, "config.json"))
		require.NoError(err)
	}
	_, err = os.Create(filepath.Join(existingDataDir, "config.json"))
	require.NoError(err)

	// also create a non-matching file name, should fail
	err = os.MkdirAll(noConfigFileDir, constants.DefaultPerms755)
	require.NoError(err)

	_, err = os.Create(filepath.Join(noConfigFileDir, "cnf.json"))
	require.NoError(err)

	var path string
	for i, d := range scanDirs {
		path = findByCommonDirs(defaultConfigFileName, scanDirs)
		// the first indexes are expected to succeed as we created files there
		switch {
		case i < 3:
			require.Equal(filepath.Join(d, defaultConfigFileName), path)
			// always remove this iteration's file as otherwise we get a false positive
			// (actually the test fails because it matches with the previous file)
			err = os.Remove(filepath.Join(d, defaultConfigFileName))
			require.NoError(err)
		case i == 3:
			require.Equal(filepath.Join(existingDataDir, defaultConfigFileName), path)
			err = os.Remove(filepath.Join(existingDataDir, defaultConfigFileName))
			require.NoError(err)
		default:
			require.Empty(path)
		}
	}
}
