// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package plugins

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/config"
	"github.com/stretchr/testify/assert"
)

// TestFindByRunningProcess starts a process and then
// runs findByRunningProcesses to check if the correct
// arguments can be extracted from the process CommandLine
func TestFindByRunningProcess(t *testing.T) {
	assert := assert.New(t)

	argWithSpace := "argWithSpace"
	argWithEqual := "argWithEqual"
	spaceValue := "/path/to/oblivion"
	equalValue := "/path/to/programmers/bliss"

	// a test with this name will be ran from within this test
	procName := "sh"

	// prepare the first test: it will be `sh -c "sleep 20; -argWithSpace /path/to/oblivion"`
	// we sleep 20 just to simulate a running process which won't just terminate before
	// we looked at the process list
	cs := []string{"-c", `sleep 20; -` + argWithSpace + ` ` + spaceValue}
	cmd := exec.Command(procName, cs...) // #nosec G204
	// start the proc async
	err := cmd.Start()
	assert.NoError(err)
	// give the process the time to actually start;
	// otherwise `findByRunningProcesses` might be done before that!
	time.Sleep(250 * time.Millisecond)
	// in a go routine (while our target backend process is running):
	// run the target function and expect the targeted argument to be found
	go func() {
		funcValue := findByRunningProcesses(procName, argWithSpace)
		assert.Equal(spaceValue, funcValue)
		// kill the process right away, we have what we wanted
		err = cmd.Process.Kill()
		assert.NoError(err)
	}()

	// wait until the command returns (which should happen because the test succeeded
	// therefore it got killed in the go routine above)
	err = cmd.Wait()
	assert.ErrorContains(err, "killed")

	// prepare the second test: it will be `sh -c "sleep 20; -argWithEqual=/path/to/programmers/bliss"`
	// we sleep 20 just to simulate a running process which won't just terminate before
	// we looked at the process list
	cs = []string{"-c", `sleep 20; -` + argWithEqual + `=` + equalValue}
	cmd2 := exec.Command(procName, cs...) // #nosec G204
	err = cmd2.Start()
	assert.NoError(err)
	// give the process the time to actually start;
	// otherwise `findByRunningProcesses` might be done before that!
	time.Sleep(250 * time.Millisecond)
	go func() {
		funcValue := findByRunningProcesses(procName, argWithEqual)
		assert.Equal(equalValue, funcValue)
		err = cmd2.Process.Kill()
		assert.NoError(err)
	}()

	err = cmd2.Wait()
	assert.ErrorContains(err, "killed")
}

func TestFindDefaultFiles(t *testing.T) {
	testDir := t.TempDir()
	assert := assert.New(t)

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
	assert.NoError(err)
	err = os.MkdirAll(scanDirs[1], constants.DefaultPerms755)
	assert.NoError(err)

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
	assert.NoError(err)

	for _, d := range scanDirs[:3] {
		_, err = os.Create(filepath.Join(d, "config.json"))
		assert.NoError(err)
	}
	_, err = os.Create(filepath.Join(existingDataDir, "config.json"))
	assert.NoError(err)

	// also create a non-matching file name, should fail
	err = os.MkdirAll(noConfigFileDir, constants.DefaultPerms755)
	assert.NoError(err)

	_, err = os.Create(filepath.Join(noConfigFileDir, "cnf.json"))
	assert.NoError(err)

	var path string
	for i, d := range scanDirs {
		path = findByCommonDirs(defaultConfigFileName, scanDirs)
		// the first indexes are expected to succeed as we created files there
		switch {
		case i < 3:
			assert.Equal(filepath.Join(d, defaultConfigFileName), path)
			// always remove this iteration's file as otherwise we get a false positive
			// (actually the test fails because it matches with the previous file)
			err = os.Remove(filepath.Join(d, defaultConfigFileName))
			assert.NoError(err)
		case i == 3:
			assert.Equal(filepath.Join(existingDataDir, defaultConfigFileName), path)
			err = os.Remove(filepath.Join(existingDataDir, defaultConfigFileName))
			assert.NoError(err)
		default:
			assert.Empty(path)
		}
	}
}
