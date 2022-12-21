// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testutils

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
	"github.com/stretchr/testify/require"
)

func CreateZip(require *require.Assertions, src string, dest string) {
	zipf, err := os.Create(dest)
	require.NoError(err)
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

	require.NoError(err)
}

func CreateTarGz(require *require.Assertions, src string, dest string, includeTopLevel bool) {
	tgz, err := os.Create(dest)
	require.NoError(err)
	defer tgz.Close()

	gw := gzip.NewWriter(tgz)
	defer gw.Close()

	tarball := tar.NewWriter(gw)
	defer tarball.Close()

	info, err := os.Stat(src)
	require.NoError(err)

	var baseDir string
	if includeTopLevel && info.IsDir() {
		baseDir = filepath.Base(src)
	} else {
		baseDir = ""
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

			if strings.TrimSuffix(header.Name, "/") == filepath.Base(src) {
				return nil
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
				require.NoError(err)
			}()
			_, err = io.Copy(tarball, file)
			return err
		})
	require.NoError(err)
}

func CreateTestArchivePath(t *testing.T, require *require.Assertions) (string, func(string)) {
	// create root test dir, will be cleaned up after test
	testDir := t.TempDir()

	// create some test dirs
	dir1 := filepath.Join(testDir, "dir1")
	dir2 := filepath.Join(testDir, "dir2")
	err := os.Mkdir(dir1, perms.ReadWriteExecute)
	require.NoError(err)
	err = os.Mkdir(dir2, perms.ReadWriteExecute)
	require.NoError(err)

	// create some (empty) files
	_, err = os.Create(filepath.Join(dir1, "gzipTest11"))
	require.NoError(err)
	_, err = os.Create(filepath.Join(dir1, "gzipTest12"))
	require.NoError(err)
	_, err = os.Create(filepath.Join(dir1, "gzipTest13"))
	require.NoError(err)
	_, err = os.Create(filepath.Join(dir2, "gzipTest21"))
	require.NoError(err)
	_, err = os.Create(filepath.Join(testDir, "gzipTest0"))
	require.NoError(err)

	// also create a binary file
	buf := make([]byte, 32)
	_, err = rand.Read(buf)
	require.NoError(err)
	binFile := filepath.Join(testDir, "binary-test-file")
	err = os.WriteFile(binFile, buf, perms.ReadWrite)
	require.NoError(err)

	// make sure the same stuff exists
	checkFunc := func(controlDir string) {
		require.DirExists(filepath.Join(controlDir, "dir1"))
		require.DirExists(filepath.Join(controlDir, "dir2"))
		require.FileExists(filepath.Join(controlDir, "dir1", "gzipTest11"))
		require.FileExists(filepath.Join(controlDir, "dir1", "gzipTest12"))
		require.FileExists(filepath.Join(controlDir, "dir1", "gzipTest13"))
		require.FileExists(filepath.Join(controlDir, "dir2", "gzipTest21"))
		require.FileExists(filepath.Join(controlDir, "gzipTest0"))
		checkBin, err := os.ReadFile(binFile)
		require.NoError(err)
		require.Equal(checkBin, buf)
	}

	return testDir, checkFunc
}
