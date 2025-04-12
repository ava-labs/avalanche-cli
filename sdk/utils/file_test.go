// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandHome(t *testing.T) {
	// Test case 1: Absolute path
	absolutePath := "/tmp/testfile.txt"
	expandedAbsolutePath := ExpandHome(absolutePath)
	if expandedAbsolutePath != absolutePath {
		t.Errorf("ExpandHome failed for absolute path: expected %s, got %s", absolutePath, expandedAbsolutePath)
	}

	// Test case 2: Relative path
	relativePath := "testfile.txt"
	expectedRelativePath := filepath.Join(".", relativePath)
	expandedRelativePath := ExpandHome(relativePath)
	if expandedRelativePath != expectedRelativePath {
		t.Errorf("ExpandHome failed for relative path: expected %s, got %s", expectedRelativePath, expandedRelativePath)
	}

	// Test case 3: Path starting with ~
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Error getting user home directory: %v", err)
	}
	tildePath := "~/testfile.txt"
	expectedTildePath := filepath.Join(homeDir, "testfile.txt")
	expandedTildePath := ExpandHome(tildePath)
	if expandedTildePath != expectedTildePath {
		t.Errorf("ExpandHome failed for path starting with ~: expected %s, got %s", expectedTildePath, expandedTildePath)
	}

	// Test case 4: Empty path
	emptyPath := ""
	expectedEmptyPath := homeDir
	expandedEmptyPath := ExpandHome(emptyPath)
	if expandedEmptyPath != expectedEmptyPath {
		t.Errorf("ExpandHome failed for empty path: expected %s, got %s", expectedEmptyPath, expandedEmptyPath)
	}
}

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	// Test that the file exists
	require.True(t, FileExists(tempFile.Name()))
	// Test that a non-existent file does not exist
	require.False(t, FileExists("non_existent_file.txt"))
}

func TestDirExists(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "testdir")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	// Test that the directory exists
	require.True(t, DirExists(tempDir))
	// Test that a non-existent directory does not exist
	require.False(t, DirExists("non_existent_dir"))
}
