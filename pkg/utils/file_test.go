// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
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
	require.Equal(t, absolutePath, expandedAbsolutePath)

	// Test case 2: Relative path
	relativePath := "testfile.txt"
	expectedRelativePath := filepath.Join(".", relativePath)
	expandedRelativePath := ExpandHome(relativePath)
	require.Equal(t, expectedRelativePath, expandedRelativePath)

	// Test case 3: Path starting with ~
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)
	tildePath := "~/testfile.txt"
	expectedTildePath := filepath.Join(homeDir, "testfile.txt")
	expandedTildePath := ExpandHome(tildePath)
	require.Equal(t, expectedTildePath, expandedTildePath)

	// Test case 4: Empty path
	emptyPath := ""
	expectedEmptyPath := homeDir
	expandedEmptyPath := ExpandHome(emptyPath)
	require.Equal(t, expectedEmptyPath, expandedEmptyPath)
}
