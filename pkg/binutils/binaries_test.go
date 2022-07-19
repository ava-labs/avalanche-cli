// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package binutils

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/stretchr/testify/assert"
)

func TestInstallZipArchive(t *testing.T) {
	assert := assert.New(t)

	archivePath, checkFunc := createTestArchivePath(t, assert)

	tmpDir := os.TempDir()
	zip := filepath.Join(tmpDir, "testFile.zip")
	defer os.Remove(zip)

	createZip(assert, archivePath, zip)

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

	archivePath, checkFunc := createTestArchivePath(t, assert)

	tmpDir := os.TempDir()
	tgz := filepath.Join(tmpDir, "testFile.tar.gz")
	defer os.Remove(tgz)

	createTarGz(assert, archivePath, tgz)

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
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

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
			if err != nil {
				return err
			}

			defer func() {
				err := file.Close()
				assert.NoError(err)
			}()
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
