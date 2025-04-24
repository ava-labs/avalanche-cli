// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package publicarchive

import (
	"archive/tar"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cavaliergopher/grab/v3"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
)

func TestNewGetter(t *testing.T) {
	endpoint := "http://example.com/file.tar"
	target := "/tmp/file.tar"

	getter, err := newGetter(endpoint, target)
	require.NoError(t, err, "newGetter should not return an error")
	require.NotNil(t, getter.client, "getter client should not be nil")
	require.NotNil(t, getter.request, "getter request should not be nil")
	require.Equal(t, endpoint, getter.request.URL().String(), "getter request URL should match the input endpoint")
}

func TestNewDownloader(t *testing.T) {
	downloader, err := NewDownloader(network.Network{ID: constants.FujiID}, logging.NewLogger("public-archive-downloader", logging.NewWrappedCore(logging.Info, os.Stdout, logging.JSON.ConsoleEncoder())))
	require.NoError(t, err, "NewDownloader should not return an error")
	require.NotNil(t, downloader.logger, "downloader logger should not be nil")
	require.NotNil(t, downloader.getter.client, "downloader getter client should not be nil")
}

func TestDownloader_Download(t *testing.T) {
	// Mock server to simulate file download
	mockData := []byte("mock file content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(mockData)
	}))
	defer server.Close()

	// Create a temporary file for download target
	tmpFile, err := os.CreateTemp("", "test-download-*")
	require.NoError(t, err, "Temporary file creation failed")
	defer os.Remove(tmpFile.Name())

	// Ensure newGetter initializes properly
	getter, err := newGetter(server.URL, tmpFile.Name())
	require.NoError(t, err, "Getter initialization failed")

	// Ensure the getter has a valid request
	require.NotNil(t, getter.request, "Getter request is nil")

	// Initialize a no-op logger to avoid output
	logger := logging.NoLog{}
	downloader := Downloader{
		getter:    getter,
		logger:    logger,
		currentOp: &sync.Mutex{},
	}

	// Test the Download functionality
	err = downloader.Download()
	require.NoError(t, err, "Download should not return an error")

	// Validate the downloaded content
	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err, "Reading downloaded file should not return an error")
	require.Equal(t, mockData, content, "Downloaded file content should match the mock data")
}

func TestDownloader_UnpackTo(t *testing.T) {
	// Create a mock tar file
	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	files := []struct {
		Name, Body string
	}{
		{"file1.txt", "This is file1"},
		{"dir/file2.txt", "This is file2"},
	}
	for _, file := range files {
		header := &tar.Header{
			Name: file.Name,
			Size: int64(len(file.Body)),
			Mode: 0o600,
		}
		require.NoError(t, tarWriter.WriteHeader(header))
		_, err := tarWriter.Write([]byte(file.Body))
		require.NoError(t, err)
	}
	require.NoError(t, tarWriter.Close())

	// Write tar file to a temporary file
	tmpTar, err := os.CreateTemp("", "test-tar-*")
	require.NoError(t, err)
	defer os.Remove(tmpTar.Name())
	_, err = tmpTar.Write(buf.Bytes())
	require.NoError(t, err)
	require.NoError(t, tmpTar.Close())

	targetDir := t.TempDir()

	logger := logging.NoLog{}
	downloader := Downloader{
		getter: Getter{
			request: &grab.Request{
				Filename: tmpTar.Name(),
			},
		},
		logger:    logger,
		currentOp: &sync.Mutex{},
	}

	err = downloader.UnpackTo(targetDir)
	require.NoError(t, err, "UnpackTo should not return an error")

	// Verify unpacked files
	for _, file := range files {
		filePath := filepath.Join(targetDir, file.Name)
		content, err := os.ReadFile(filePath)
		require.NoError(t, err, fmt.Sprintf("Reading file %s should not return an error", file.Name))
		require.Equal(t, file.Body, string(content), fmt.Sprintf("File content for %s should match", file.Name))
	}
}

func TestDownloader_EndToEnd(t *testing.T) {
	// Set up a temporary directory for testing
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "extracted_files")

	// Configure the test network (Fuji in this case)
	net := network.Network{ID: constants.FujiID}

	// Step 1: Create the downloader
	downloader, err := NewDownloader(net, logging.NewLogger("public-archive-downloader", logging.NewWrappedCore(logging.Debug, os.Stdout, logging.JSON.ConsoleEncoder())))
	require.NoError(t, err, "Failed to initialize downloader")

	// Step 2: Start the download
	t.Log("Starting download...")
	err = downloader.Download()
	require.NoError(t, err, "Download failed")

	// Step 3: Unpack the downloaded archive
	t.Log("Unpacking downloaded archive...")
	err = downloader.UnpackTo(targetDir)
	require.NoError(t, err, "Failed to unpack archive")

	// Step 4: Validate the extracted files
	t.Log("Validating extracted files...")
	fileInfo, err := os.Stat(targetDir)
	require.NoError(t, err, "Extracted directory does not exist")
	require.True(t, fileInfo.IsDir(), "Extracted path is not a directory")

	// Check that at least one file is extracted
	extractedFiles, err := os.ReadDir(targetDir)
	require.NoError(t, err, "Failed to read extracted directory contents")
	require.NotEmpty(t, extractedFiles, "No files extracted from archive")

	// Step 5: Clean up (optional since TempDir handles this automatically)
	t.Log("Test completed successfully!")
}
