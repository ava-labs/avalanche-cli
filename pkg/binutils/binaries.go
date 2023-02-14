// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package binutils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

var (
	// interface compliance
	_ PluginBinaryDownloader = (*pluginBinaryDownloader)(nil)
	_ BinaryChecker          = (*binaryChecker)(nil)
)

type PluginBinaryDownloader interface {
	InstallVM(vmID, vmBin string) error
	UpgradeVM(vmID, vmBin string) error
	RemoveVM(vmID string) error
}

type BinaryChecker interface {
	ExistsWithVersion(name, binaryPrefix, version string) (bool, error)
}

type (
	binaryChecker          struct{}
	pluginBinaryDownloader struct {
		app *application.Avalanche
	}
)

func NewBinaryChecker() BinaryChecker {
	return &binaryChecker{}
}

func NewPluginBinaryDownloader(app *application.Avalanche) PluginBinaryDownloader {
	return &pluginBinaryDownloader{
		app: app,
	}
}

// Sanitize archive file pathing from "G305: Zip Slip vulnerability"
func sanitizeArchivePath(d, t string) (v string, err error) {
	v = filepath.Join(d, t)
	if strings.HasPrefix(v, filepath.Clean(d)) {
		return v, nil
	}

	return "", fmt.Errorf("%s: %s", "content filepath is tainted", t)
}

// InstallArchive installs the binary archive downloaded
func InstallArchive(ext string, archive []byte, binDir string) error {
	// create binDir if it doesn't exist
	if err := os.MkdirAll(binDir, constants.DefaultPerms755); err != nil {
		return err
	}

	if ext == "zip" {
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

	if err := os.MkdirAll(binDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("failed to create app binary directory: %w", err)
	}

	// Closure to address file descriptors issue, uses Close to to not leave open descriptors
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed opening zip file: %w", err)
		}

		// check for zip slip
		path, err := sanitizeArchivePath(binDir, f.Name)
		if err != nil {
			return err
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

			_, err = io.CopyN(f, rc, maxCopy)
			if err != nil && !errors.Is(err, io.EOF) {
				return fmt.Errorf("failed writing zip file entry to disk: %w", err)
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		if err := rc.Close(); err != nil {
			return err
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
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return fmt.Errorf("failed reading next tar entry: %w", err)
		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		// check for zip slip
		target, err := sanitizeArchivePath(binDir, header.Name)
		if err != nil {
			return err
		}

		// check the file type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, constants.DefaultPerms755); err != nil {
					return fmt.Errorf("failed creating directory from tar entry %w", err)
				}
			}
		// if it's a file create it
		case tar.TypeReg:
			// if the containing directory doesn't exist yet, create it
			containingDir := filepath.Dir(target)
			if err := os.MkdirAll(containingDir, constants.DefaultPerms755); err != nil {
				return fmt.Errorf("failed creating directory from tar entry %w", err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed opening new file from tar entry %w", err)
			}
			// copy over contents
			if _, err := io.CopyN(f, tarReader, maxCopy); err != nil && !errors.Is(err, io.EOF) {
				return fmt.Errorf("failed writing tar entry contents to disk: %w", err)
			}
			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}

// ExistsWithVersion returns true if the supplied binary is installed with the supplied version
func (*binaryChecker) ExistsWithVersion(binDir, binPrefix, version string) (bool, error) {
	match, err := filepath.Glob(filepath.Join(binDir, binPrefix) + version)
	if err != nil {
		return false, err
	}
	return len(match) != 0, nil
}

func (pbd *pluginBinaryDownloader) InstallVM(vmID, vmBin string) error {
	// target of VM install
	binaryPath := filepath.Join(pbd.app.GetPluginsDir(), vmID)

	// check if binary is already present, this should never happen
	if _, err := os.Stat(binaryPath); err == nil {
		return errors.New("vm binary already exists, invariant broken")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := CopyFile(vmBin, binaryPath); err != nil {
		return fmt.Errorf("failed copying vm to plugin dir: %w", err)
	}
	return nil
}

func (pbd *pluginBinaryDownloader) UpgradeVM(vmID, vmBin string) error {
	// target of VM install
	binaryPath := filepath.Join(pbd.app.GetPluginsDir(), vmID)

	// check if binary is already present, it should already exist
	if _, err := os.Stat(binaryPath); !errors.Is(err, os.ErrNotExist) {
		return errors.New("vm binary does not exist, are you sure this Subnet is ready to upgrade?")
	}

	// overwrite existing file with new binary
	if err := CopyFile(vmBin, binaryPath); err != nil {
		return fmt.Errorf("failed copying vm to plugin dir: %w", err)
	}
	return nil
}

func (pbd *pluginBinaryDownloader) RemoveVM(vmID string) error {
	// target of VM install
	binaryPath := filepath.Join(pbd.app.GetPluginsDir(), vmID)

	// check if binary is already present, this should never happen
	if _, err := os.Stat(binaryPath); errors.Is(err, os.ErrNotExist) {
		return errors.New("vm binary does not exist")
	} else if err != nil {
		return err
	}

	if err := os.Remove(binaryPath); err != nil {
		return fmt.Errorf("failed deleting plugin: %w", err)
	}
	return nil
}
