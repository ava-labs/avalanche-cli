// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"os"
	"path/filepath"
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
