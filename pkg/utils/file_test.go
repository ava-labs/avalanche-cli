// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"os"
	"path/filepath"
	"testing"
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

// createTempGoMod creates a temporary go.mod file with the provided content.
func createTempGoMod(t *testing.T, content string) string {
	t.Helper()
	file, err := os.CreateTemp("", "go.mod")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	return file.Name()
}

// TestReadGoVersion tests all scenarios in one function using sub-tests.
func TestReadGoVersion(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tempFile := createTempGoMod(t, "module example.com/test\n\ngo 1.18\n")
		defer os.Remove(tempFile) // Clean up the temp file

		version, err := ReadGoVersion(tempFile)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expectedVersion := "1.18"
		if version != expectedVersion {
			t.Errorf("expected version %s, got %s", expectedVersion, version)
		}
	})

	t.Run("NoVersion", func(t *testing.T) {
		tempFile := createTempGoMod(t, "module example.com/test\n")
		defer os.Remove(tempFile)

		_, err := ReadGoVersion(tempFile)
		if err == nil {
			t.Fatalf("expected an error, but got none")
		}
	})

	t.Run("InvalidFile", func(t *testing.T) {
		_, err := ReadGoVersion("nonexistent-go.mod")
		if err == nil {
			t.Fatalf("expected an error for nonexistent file, but got none")
		}
	})
}
