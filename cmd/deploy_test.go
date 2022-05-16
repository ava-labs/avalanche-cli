// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestExistsWithLatestVersion(t *testing.T) {
	assert := assert.New(t)
	tmpDir := t.TempDir()
	bc := NewBinaryChecker()

	exists, latest, err := bc.ExistsWithLatestVersion(tmpDir)
	assert.NoError(err)
	assert.False(exists)
	assert.Empty(latest)

	fake := filepath.Join(tmpDir, "anything")
	err = os.Mkdir(fake, perms.ReadWriteExecute)
	assert.NoError(err)
	exists, latest, err = bc.ExistsWithLatestVersion(tmpDir)
	assert.NoError(err)
	assert.False(exists)
	assert.Empty(latest)

	avagoOnly := filepath.Join(tmpDir, "avalanchego")
	err = os.Mkdir(avagoOnly, perms.ReadWriteExecute)
	assert.NoError(err)
	exists, latest, err = bc.ExistsWithLatestVersion(tmpDir)
	assert.NoError(err)
	assert.False(exists)
	assert.Empty(latest)

	existsOneOnly := filepath.Join(tmpDir, "avalanchego-v1.7.10")
	err = os.Mkdir(existsOneOnly, perms.ReadWriteExecute)
	assert.NoError(err)
	exists, latest, err = bc.ExistsWithLatestVersion(tmpDir)
	assert.NoError(err)
	assert.True(exists)
	assert.Equal(existsOneOnly, latest)

	ver1 := filepath.Join(tmpDir, "avalanchego-v1.8.0")
	ver2 := filepath.Join(tmpDir, "avalanchego-v1.18.0")
	ver3 := filepath.Join(tmpDir, "avalanchego-v1.8.1")
	ver4 := filepath.Join(tmpDir, "avalanchego-v0.8.0")
	ver5 := filepath.Join(tmpDir, "avalanchego-v0.88.0")
	ver6 := filepath.Join(tmpDir, "avalanchego-v0.1.0")
	ver7 := filepath.Join(tmpDir, "avalanchego-v0.11.0")
	ver8 := filepath.Join(tmpDir, "avalanchego-0.11.0")

	err = os.Mkdir(ver1, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver2, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver3, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver4, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver5, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver6, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver7, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver8, perms.ReadWriteExecute)
	assert.NoError(err)

	exists, latest, err = bc.ExistsWithLatestVersion(tmpDir)
	assert.NoError(err)
	assert.True(exists)
	assert.Equal(ver2, latest)
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

func TestInstallZipArchive(t *testing.T) {
	assert := assert.New(t)

	archivePath, checkFunc := createTestArchivePath(t, assert)

	tmpDir := os.TempDir()
	zip := filepath.Join(tmpDir, "testFile.zip")
	defer os.Remove(zip)

	createZip(assert, archivePath, zip)

	// can't use t.TempDir here as that returns the same dir
	installDir, err := ioutil.TempDir(tmpDir, "zip-test-dir")
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

	archivePath, checkFunc := createTestArchivePath(t, assert)

	tmpDir := os.TempDir()
	tgz := filepath.Join(tmpDir, "testFile.tar.gz")
	defer os.Remove(tgz)

	createTarGz(assert, archivePath, tgz)

	// can't use t.TempDir here as that returns the same dir
	installDir, err := ioutil.TempDir(tmpDir, "gzip-test-dir")
	assert.NoError(err)
	defer os.RemoveAll(installDir)

	tgzBytes, err := os.ReadFile(tgz)
	assert.NoError(err)

	err = installTarGzArchive(tgzBytes, installDir)
	assert.NoError(err)

	checkFunc(archivePath)
}

func createZip(assert *assert.Assertions, src string, dest string) {
	zipf, err := os.Create(dest)
	assert.NoError(err)
	defer zipf.Close()

	zipWriter := zip.NewWriter(zipf)
	defer zipWriter.Close()

	// 2. Go through all the files of the source
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 3. Create a local file header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// set compression
		header.Method = zip.Deflate

		// 4. Set relative path of a file as the header name
		header.Name, err = filepath.Rel(filepath.Dir(src), path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			header.Name += "/"
		}

		// 5. Create writer for the file header and save content of the file
		headerWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(headerWriter, f)
		return err
	})

	assert.NoError(err)
}

func createTarGz(assert *assert.Assertions, src string, dest string) {
	tgz, err := os.Create(dest)
	assert.NoError(err)
	defer tgz.Close()

	gw := gzip.NewWriter(tgz)
	defer gw.Close()

	tarball := tar.NewWriter(gw)
	defer tarball.Close()

	info, err := os.Stat(src)
	assert.NoError(err)

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(src)
	}

	err = filepath.Walk(src,
		func(path string, info os.FileInfo, err error) error {
			header, err := tar.FileInfoHeader(info, info.Name())

			if baseDir != "" {
				header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, src))
			}

			if err := tarball.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			defer file.Close()
			_, err = io.Copy(tarball, file)
			return err
		})
	assert.NoError(err)
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

func getTestClientFunc() (client.Client, error) {
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
