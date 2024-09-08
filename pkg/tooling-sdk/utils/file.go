// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/tooling-sdk/constants"
)

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

// DirectoryExists checks if a directory exists.
//
// dirName: the name of the directory to check.
// bool: returns true if the directory exists, false otherwise.
func DirectoryExists(dirName string) bool {
	info, err := os.Stat(dirName)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
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

// RemoteComposeFile returns the path to the remote docker-compose file
func GetRemoteComposeFile() string {
	return filepath.Join(constants.CloudNodeCLIConfigBasePath, "services", "docker-compose.yml")
}

// GetRemoteComposeServicePath returns the path to the remote service directory
func GetRemoteComposeServicePath(serviceName string, dirs ...string) string {
	servicePrefix := filepath.Join(constants.CloudNodeCLIConfigBasePath, "services", serviceName)
	return filepath.Join(append([]string{servicePrefix}, dirs...)...)
}
