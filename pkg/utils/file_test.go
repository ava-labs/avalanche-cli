// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/utils/logging"

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
	expectedRelativePath, _ := filepath.Abs(relativePath)
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

// createTemp creates a temporary file with the provided name prefix and content.
func createTemp(t *testing.T, namePrefix string, content string) string {
	t.Helper()
	file, err := os.CreateTemp("", namePrefix)
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
		tempFile := createTemp(t, "go.mod", "module example.com/test\n\ngo 1.23\n")
		defer os.Remove(tempFile) // Clean up the temp file

		version, err := ReadGoVersion(tempFile)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expectedVersion := "1.23"
		if version != expectedVersion {
			t.Errorf("expected version %s, got %s", expectedVersion, version)
		}
	})

	t.Run("NoVersion", func(t *testing.T) {
		tempFile := createTemp(t, "go.mod", "module example.com/test\n")
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

func TestSetupExecFile(t *testing.T) {
	srcContent := "src content"
	destContent := "dest content"
	t.Run("Src does not exists", func(t *testing.T) {
		src := createTemp(t, "testexecfile", srcContent)
		dest := createTemp(t, "testexecfile", destContent)
		err := os.Remove(src)
		require.NoError(t, err)
		require.Equal(t, false, FileExists(src))
		require.Equal(t, true, FileExists(dest))
		require.Equal(t, false, IsExecutable(dest))
		err = SetupExecFile(logging.NoLog{}, src, dest)
		require.Error(t, err)
		content, err := os.ReadFile(dest)
		require.NoError(t, err)
		require.Equal(t, true, FileExists(dest))
		require.Equal(t, false, IsExecutable(dest))
		require.Equal(t, destContent, string(content))
	})
	t.Run("Dest does not exists", func(t *testing.T) {
		src := createTemp(t, "testexecfile", srcContent)
		dest := createTemp(t, "testexecfile", destContent)
		err := os.Remove(dest)
		require.NoError(t, err)
		require.Equal(t, false, FileExists(dest))
		require.Equal(t, false, IsExecutable(dest))
		err = SetupExecFile(logging.NoLog{}, src, dest)
		require.NoError(t, err)
		content, err := os.ReadFile(dest)
		require.NoError(t, err)
		require.Equal(t, true, FileExists(dest))
		require.Equal(t, true, IsExecutable(dest))
		require.Equal(t, srcContent, string(content))
	})
	t.Run("Dest is not executable", func(t *testing.T) {
		src := createTemp(t, "testexecfile", srcContent)
		dest := createTemp(t, "testexecfile", destContent)
		require.Equal(t, true, FileExists(dest))
		require.Equal(t, false, IsExecutable(dest))
		err := SetupExecFile(logging.NoLog{}, src, dest)
		require.NoError(t, err)
		content, err := os.ReadFile(dest)
		require.NoError(t, err)
		require.Equal(t, true, FileExists(dest))
		require.Equal(t, true, IsExecutable(dest))
		require.Equal(t, srcContent, string(content))
	})
	t.Run("Dest is already executable", func(t *testing.T) {
		src := createTemp(t, "testexecfile", srcContent)
		dest := createTemp(t, "testexecfile", destContent)
		err := os.Chmod(dest, constants.DefaultPerms755)
		require.NoError(t, err)
		require.Equal(t, true, FileExists(dest))
		require.Equal(t, true, IsExecutable(dest))
		err = SetupExecFile(logging.NoLog{}, src, dest)
		require.NoError(t, err)
		content, err := os.ReadFile(dest)
		require.NoError(t, err)
		require.Equal(t, true, FileExists(dest))
		require.Equal(t, true, IsExecutable(dest))
		require.Equal(t, destContent, string(content))
	})
}
