// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/mocks"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeployToLocal(t *testing.T) {
	assert := assert.New(t)

	log = logging.NoLog{}
	// fake-return true simulating the process is running
	procChecker := &mocks.ProcessChecker{}
	procChecker.On("IsServerProcessRunning").Return(true, nil)

	// create a dummy plugins dir, deploy will check it exists
	binChecker := &mocks.BinaryChecker{}
	tmpDir := t.TempDir()
	err := os.Mkdir(filepath.Join(tmpDir, "plugins"), perms.ReadWriteExecute)
	assert.NoError(err)

	// create a dummy avalanchego file, deploy will check it exists
	f, err := os.Create(filepath.Join(tmpDir, "avalanchego"))
	assert.NoError(err)
	defer func() {
		_ = f.Close()
	}()

	binChecker.On("ExistsWithLatestVersion", mock.AnythingOfType("string")).Return(true, tmpDir, nil)

	binDownloader := &mocks.BinaryDownloader{}
	binDownloader.On("Download", mock.AnythingOfType("ids.ID"), mock.AnythingOfType("string")).Return(nil)

	testDeployer := &subnetDeployer{
		procChecker:         procChecker,
		binChecker:          binChecker,
		getClientFunc:       getTestClientFunc,
		binaryDownloader:    binDownloader,
		healthCheckInterval: 500 * time.Millisecond,
	}

	// create a dummy genesis file, deploy will check it exists
	testGenesis, err := os.CreateTemp(tmpDir, "test-genesis.json")
	assert.NoError(err)

	// test actual deploy
	err = testDeployer.deployToLocalNetwork("test", testGenesis.Name())
	assert.NoError(err)
}

func TestGetLatestAvagoVersion(t *testing.T) {
	assert := assert.New(t)
	testVersion := "v1.99.9999"
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := fmt.Sprintf(`{"some":"unimportant","fake":"data","tag_name":"%s","tag_name_was":"what we are interested in"}`, testVersion)
		_, err := w.Write([]byte(resp))
		assert.NoError(err)
	})
	s := httptest.NewServer(testHandler)
	defer s.Close()

	v, err := getLatestAvagoVersion(s.URL)
	assert.NoError(err)
	assert.Equal(v, testVersion)
}

func TestInstallGzipArchive(t *testing.T) {
	assert := assert.New(t)

	archivePath, checkFunc := createTestArchivePath(t, assert)

	tmpDir := os.TempDir()
	tgz := filepath.Join(tmpDir, "testFile.tar.gz")
	defer os.Remove(tgz)
	// TODO: Maybe we could just have a test tar.gz file around
	// (tar should be installed on the system, but you never know...)
	// That's why we can't test zip easily as that is most probably not installed
	cmd := exec.Command("tar", "-zcf", tgz, archivePath)
	err := cmd.Run()
	assert.NoError(err)

	// can't use t.TempDir here as that returns the same dir
	installDir, err := ioutil.TempDir(tmpDir, "gzip-test-dir")
	assert.NoError(err)
	defer os.RemoveAll(installDir)

	tgzBytes, err := os.ReadFile(tgz)
	assert.NoError(err)

	err = installTarGzArchive(tgzBytes, installDir)
	assert.NoError(err)

	controlDir := filepath.Join(installDir, archivePath)
	checkFunc(controlDir)
}

func createTestArchivePath(t *testing.T, assert *assert.Assertions) (string, func(string)) {
	// create root test dir, will be cleaned up after test
	testDir := t.TempDir()

	// create some test dirs
	dir1 := filepath.Join(testDir, "dir1")
	dir2 := filepath.Join(testDir, "dir2")
	err := os.Mkdir(dir1, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(dir2, perms.ReadWriteExecute)
	assert.NoError(err)

	// create some (empty) files
	_, err = os.Create(filepath.Join(dir1, "gzipTest11"))
	assert.NoError(err)
	_, err = os.Create(filepath.Join(dir1, "gzipTest12"))
	assert.NoError(err)
	_, err = os.Create(filepath.Join(dir1, "gzipTest13"))
	assert.NoError(err)
	_, err = os.Create(filepath.Join(dir2, "gzipTest21"))
	assert.NoError(err)
	_, err = os.Create(filepath.Join(testDir, "gzipTest0"))
	assert.NoError(err)

	// also create a binary file
	buf := make([]byte, 32)
	_, err = rand.Read(buf)
	assert.NoError(err)
	binFile := filepath.Join(testDir, "binary-test-file")
	err = os.WriteFile(binFile, buf, perms.ReadWrite)
	assert.NoError(err)

	// make sure the same stuff exists
	checkFunc := func(controlDir string) {
		assert.DirExists(filepath.Join(controlDir, "dir1"))
		assert.DirExists(filepath.Join(controlDir, "dir2"))
		assert.FileExists(filepath.Join(controlDir, "dir1", "gzipTest11"))
		assert.FileExists(filepath.Join(controlDir, "dir1", "gzipTest12"))
		assert.FileExists(filepath.Join(controlDir, "dir1", "gzipTest13"))
		assert.FileExists(filepath.Join(controlDir, "dir2", "gzipTest21"))
		assert.FileExists(filepath.Join(controlDir, "gzipTest0"))
		checkBin, err := os.ReadFile(binFile)
		assert.NoError(err)
		assert.Equal(checkBin, buf)
	}

	return testDir, checkFunc
}

func getTestClientFunc(logLevel string, endpoint string, timeout time.Duration) (client.Client, error) {
	c := &mocks.Client{}
	fakeStartResponse := &rpcpb.StartResponse{}
	c.On("Start", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fakeStartResponse, nil)
	fakeHealthResponse := &rpcpb.HealthResponse{
		ClusterInfo: &rpcpb.ClusterInfo{
			NodeInfos: map[string]*rpcpb.NodeInfo{
				"testNode1": {
					Name: "testNode1",
					Uri:  "http://fake.localhost:12345",
				},
				"testNode2": {
					Name: "testNode2",
					Uri:  "http://fake.localhost:12345",
				},
			},
			CustomVms: map[string]*rpcpb.CustomVmInfo{
				"vm1": {
					BlockchainId: "abcd",
				},
				"vm2": {
					BlockchainId: "efgh",
				},
			},
		},
	}
	c.On("Health", mock.Anything).Return(fakeHealthResponse, nil)
	c.On("Close").Return(nil)
	return c, nil
}
