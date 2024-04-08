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
}
