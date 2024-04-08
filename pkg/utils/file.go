// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
)

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
