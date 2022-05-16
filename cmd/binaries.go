// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/coreos/go-semver/semver"
)

type PluginBinaryDownloader interface {
	Download(ids.ID, string) error
}

type BinaryChecker interface {
	ExistsWithLatestVersion(name string) (bool, string, error)
}

type (
	binaryChecker          struct{}
	pluginBinaryDownloader struct{}
)

func NewBinaryChecker() BinaryChecker {
	return &binaryChecker{}
}

func newPluginBinaryDownloader() PluginBinaryDownloader {
	return &pluginBinaryDownloader{}
}

// installArchive installs the binary archive downloaded in a os-dependent way
func installArchive(goos string, archive []byte, binDir string) error {
	if goos == "darwin" || goos == "windows" {
		return installZipArchive(archive, binDir)
	}
	return installTarGzArchive(archive, binDir)
}

// installZipArchive expects a byte stream of a zip file
func installZipArchive(zipfile []byte, binDir string) error {
	bytesReader := bytes.NewReader(zipfile)
	zipReader, err := zip.NewReader(bytesReader, int64(len(zipfile)))
	if err != nil {
		return fmt.Errorf("failed creating zip reader from binary stream: %w", err)
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create app binary directory: %w", err)
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed opening zip file: %w", err)
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(binDir, f.Name)
		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(binDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return fmt.Errorf("failed creating directory from zip entry: %w", err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), f.Mode()); err != nil {
				return fmt.Errorf("failed creating file from zip entry: %w", err)
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return fmt.Errorf("failed opening file from zip entry: %w", err)
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return fmt.Errorf("failed writing zip file entry to disk: %w", err)
			}
		}
		return nil
	}

	for _, f := range zipReader.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

// installTarGzArchive expects a byte array in targz format
func installTarGzArchive(targz []byte, binDir string) error {
	byteReader := bytes.NewReader(targz)
	uncompressedStream, err := gzip.NewReader(byteReader)
	if err != nil {
		return fmt.Errorf("failed creating gzip reader from avalanchego binary stream: %w", err)
	}

	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()
		switch {
		// if no more files are found return
		case err == io.EOF:
			return nil
		case err != nil:
			return fmt.Errorf("failed reading next tar entry: %w", err)
		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}
		// the target location where the dir/file should be created
		target := filepath.Join(binDir, header.Name)

		// check the file type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return fmt.Errorf("failed creating directory from tar entry %w", err)
				}
			}
		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed opening new file from tar entry %w", err)
			}
			// copy over contents
			if _, err := io.Copy(f, tarReader); err != nil {
				return fmt.Errorf("failed writing tar entry contents to disk: %w", err)
			}
			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

// ExistsWithLatestVersion returns true if avalanchego can be found and at what path
// or false, if it can not be found (or an error if applies)
func (abc *binaryChecker) ExistsWithLatestVersion(binDir string) (bool, string, error) {
	// TODO this still has loads of potential pit falls
	// Should prob check for existing binary and plugin dir too
	startsWith := "avalanchego-v"
	match, err := filepath.Glob(filepath.Join(binDir, startsWith) + "*")
	if err != nil {
		return false, "", err
	}
	var latest string
	switch len(match) {
	case 0:
		return false, "", nil
	case 1:
		latest = match[0]
	default:
		var semVers semver.Versions
		for _, v := range match {
			base := filepath.Base(v)
			newv, err := semver.NewVersion(base[len(startsWith):])
			if err != nil {
				// ignore this one, it might be in an unexpected format
				// e.g. a dir which has nothing to do with this
				continue
			}
			semVers = append(semVers, newv)
		}

		sort.Sort(sort.Reverse(semVers))
		choose := fmt.Sprintf("v%s", semVers[0])
		for _, m := range match {
			if strings.Contains(m, choose) {
				latest = m
				break
			}
		}
	}
	return true, latest, nil
}

// getVMBinary downloads the binary from the binary server URL
func (d *pluginBinaryDownloader) Download(id ids.ID, pluginDir string) error {
	vmID := id.String()
	binaryPath := filepath.Join(pluginDir, vmID)
	info, err := os.Stat(binaryPath)
	if err == nil {
		if !info.IsDir() {
			log.Debug("binary already exists, skipping download")
		}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	log.Info("VM binary does not exist locally, starting download...")

	base, err := url.Parse(binaryServerURL)
	if err != nil {
		return err
	}

	// Path params
	// base.Path += "this will get automatically encoded"

	// Query params
	params := url.Values{}
	params.Add("vmid", vmID)
	base.RawQuery = params.Encode()

	log.Debug("starting download from %s...\n\n", base.String())

	resp, err := http.Get(base.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		log.Debug("download successful. installing binary...")
		return installBinary(bodyBytes, binaryPath)

	} else {
		return fmt.Errorf("downloading binary failed, status code: %d", resp.StatusCode)
	}
}

// installBinary writes the binary as a byte array to the specified path
func installBinary(binary []byte, binaryPath string) error {
	if err := os.WriteFile(binaryPath, binary, 0o755); err != nil {
		return err
	}
	log.Info("binary installed. ready to go.")
	return nil
}
