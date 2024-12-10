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
	logLevel := logging.Info

	downloader, err := NewDownloader(network.Network{ID: constants.FujiID}, logLevel)
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

	tmpFile, err := os.CreateTemp("", "test-download-*")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	getter, err := newGetter(server.URL, tmpFile.Name())
	require.NoError(t, err)

	logger := logging.NoLog{}
	downloader := Downloader{
		getter: getter,
		logger: logger,
		mutex:  &sync.Mutex{},
	}

	err = downloader.Download()
	require.NoError(t, err, "Download should not return an error")
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
		logger: logger,
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
