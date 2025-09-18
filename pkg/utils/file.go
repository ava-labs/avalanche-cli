// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"
	"github.com/ava-labs/avalanchego/utils/logging"

	"go.uber.org/zap"
	"golang.org/x/mod/modfile"
)

func NonEmptyDirectory(dirName string) (bool, error) {
	if !sdkutils.DirExists(dirName) {
		return false, fmt.Errorf("%s is not a directory", dirName)
	}
	files, err := os.ReadDir(dirName)
	if err != nil {
		return false, err
	}
	return len(files) != 0, nil
}

// FileExists checks if a file exists.
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// IsExecutable checks if a file is executable.
func IsExecutable(filename string) bool {
	if !FileExists(filename) {
		return false
	}
	info, _ := os.Stat(filename)
	// A file perm can be seen as a 9-bit sequence mapping to: rwxrwxrwx
	// read-write-execute for owner, group and everybody.
	// 0o100 is the bit mask that, when applied with the bitwise-AND operator &,
	// results in != 0 state whenever the file can be executed by the owner.
	return info.Mode()&0o100 != 0
}

// UserHomePath returns the absolute path of a file located in the user's home directory.
func UserHomePath(filePath ...string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(filePath...)
	}
	fullPath := append([]string{home}, filePath...)
	return filepath.Join(fullPath...)
}

// ExpandHome expands ~ symbol to home directory
func ExpandHome(path string) string {
	if path == "" {
		home, _ := os.UserHomeDir()
		return home
	}
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	path, _ = filepath.Abs(path)
	return path
}

// ReplaceUserHomeWithTilde replaces user home directory with ~
func ReplaceUserHomeWithTilde(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		path = "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// FileCopy copies a file from src to dst using streaming copy for memory efficiency.
func FileCopy(src string, dst string) error {
	if !FileExists(src) {
		return fmt.Errorf("source file does not exist")
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if cerr := dstFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// Stream copy from source to destination
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err = dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	// Set proper permissions
	if err = dstFile.Chmod(constants.WriteReadReadPerms); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// SetupExecFile copies a file into destination and set it to have exec perms,
// if destination either does not exists, or is not executable
func SetupExecFile(
	log logging.Logger,
	src string,
	dst string,
) error {
	if !IsExecutable(dst) {
		if FileExists(dst) {
			log.Error(
				"binary was not properly installed on a previous CLI execution",
				zap.String("binary-path", dst),
			)
		}
		// Either it was never installed, or it was partially done (copy or chmod
		// failure)
		// As the file is not executable, there is no risk of encountering text file busy
		// error during copy, because that happens when the binary is being executed.
		if err := FileCopy(src, dst); err != nil {
			return err
		}
		if err := os.Chmod(dst, constants.DefaultPerms755); err != nil {
			return err
		}
	}
	return nil
}

// ReadFile reads a file and returns the contents as a string
func ReadFile(filePath string) (string, error) {
	filePath = ExpandHome(filePath)
	if !FileExists(filePath) {
		return "", fmt.Errorf("file does not exist")
	} else {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

// WriteStringToFile writes a string to a file
func WriteStringToFile(filePath string, data string) error {
	filePath = ExpandHome(filePath)
	return os.WriteFile(filePath, []byte(data), constants.WriteReadReadPerms)
}

// SizeInKB returns the size of a file or directory.
func SizeInKB(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// RemoteComposeFile returns the path to the remote docker-compose file
func GetRemoteComposeFile() string {
	return filepath.Join(constants.CloudNodeCLIConfigBasePath, "services", "docker-compose.yml")
}

// GetRemoteComposeServicePath returns the path to the remote service directory
func GetRemoteComposeServicePath(serviceName string, dirs ...string) string {
	servicePrefix := filepath.Join(constants.CloudNodeCLIConfigBasePath, "services", serviceName)
	return filepath.Join(append([]string{servicePrefix}, dirs...)...)
}

// ReadGoVersion reads the Go version from the go.mod file
func ReadGoVersion(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	modFile, err := modfile.Parse(filePath, data, nil)
	if err != nil {
		return "", err
	}
	if modFile.Go != nil {
		return modFile.Go.Version, nil
	}
	return "", fmt.Errorf("go version not found in %s", filePath)
}
