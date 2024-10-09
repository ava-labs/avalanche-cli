// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"golang.org/x/mod/modfile"
)

func NonEmptyDirectory(dirName string) (bool, error) {
	if !DirectoryExists(dirName) {
		return false, fmt.Errorf("%s is not a directory", dirName)
	}
	files, err := os.ReadDir(dirName)
	if err != nil {
		return false, err
	}
	return len(files) != 0, nil
}

func DirectoryExists(dirName string) bool {
	info, err := os.Stat(dirName)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
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
	return info.Mode()&0x0100 != 0
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
	return path
}

// FileCopy copies a file from src to dst.
func FileCopy(src string, dst string) error {
	if !FileExists(src) {
		return fmt.Errorf("source file does not exist")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, constants.WriteReadReadPerms)
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

// Size returns the size of a file or directory.
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
	modFile, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		return "", err
	}
	if modFile.Go != nil {
		return modFile.Go.Version, nil
	}
	return "", fmt.Errorf("go version not found in %s", filePath)
}
