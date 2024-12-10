package publicarchive

import (
	"archive/tar"
	"fmt"
	"os"
)

// IsEmpty returns true if the Downloader is empty and not initialized
func (d Downloader) IsEmpty() bool {
	return d.getter.client == nil
}

// GetDownloadSize returns the size of the download
func (d Downloader) GetDownloadSize() int64 {
	return d.getter.size
}

// GetCurrentProgress returns the current download progress
func (d Downloader) GetCurrentProgress() int64 {
	return d.getter.bytesComplete
}

func isValidTar(filePath string) (bool, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create a tar reader
	tarReader := tar.NewReader(file)

	// Try reading the first header to validate the archive
	_, err = tarReader.Next()
	if err != nil {
		return false, nil // Not a valid tar file if it cannot read a header
	}

	// If we successfully read a header, it's a valid tar file
	return true, nil
}
