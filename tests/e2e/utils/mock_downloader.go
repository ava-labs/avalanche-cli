// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"os"
)

// MockDownloader is a simple mock implementation of the downloader interface
// that returns the contents of a local file
type MockDownloader struct {
	filePath string
}

// NewMockDownloader creates a new MockDownloader that will return the contents
// of the specified file
func NewMockDownloader(filePath string) *MockDownloader {
	return &MockDownloader{
		filePath: filePath,
	}
}

// Download implements the downloader interface by reading the contents of the
// specified file
func (m *MockDownloader) Download(_ string) ([]byte, error) {
	return os.ReadFile(m.filePath)
}
