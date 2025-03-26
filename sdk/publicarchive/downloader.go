// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package publicarchive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ava-labs/avalanche-cli/sdk/constants"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	avagoConstants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/cavaliergopher/grab/v3"
	"go.uber.org/zap"
)

const (
	updateInterval = 500 * time.Millisecond
	maxFileSize    = 10 * 1024 * 1024 * 1024 // 10GB per file
	// public archive
	PChainArchiveFuji = "https://avalanchego-public-shared-database.avax-test.network/testnet/p-chain/avalanchego/data-tar/latest.tar"
)

type Getter struct {
	client        *grab.Client
	request       *grab.Request
	size          int64
	bytesComplete int64
	mutex         *sync.RWMutex
}

type Downloader struct {
	getter    Getter
	logger    logging.Logger
	currentOp *sync.Mutex
}

// newGetter returns a new Getter
func newGetter(endpoint string, target string) (Getter, error) {
	if request, err := grab.NewRequest(target, endpoint); err != nil {
		return Getter{}, err
	} else {
		return Getter{
			client:        grab.NewClient(),
			request:       request,
			size:          0,
			bytesComplete: 0,
			mutex:         &sync.RWMutex{},
		}, nil
	}
}

// TODO: add timeout control based on contexts
// TODO: improve locks to provide graceful recovery under signals such as SIGTERM

// NewDownloader returns a new Downloader
// network: the network to download from ( fuji only).
// target: the path to download to
// logLevel: the log level
func NewDownloader(
	network network.Network,
	logger logging.Logger,
) (Downloader, error) {
	tmpFile, err := os.CreateTemp("", "avalanche-cli-public-archive-*")
	if err != nil {
		return Downloader{}, err
	}

	switch network.ID {
	case avagoConstants.FujiID:
		if getter, err := newGetter(PChainArchiveFuji, tmpFile.Name()); err != nil {
			return Downloader{}, err
		} else {
			return Downloader{
				getter:    getter,
				logger:    logger,
				currentOp: &sync.Mutex{},
			}, nil
		}
	default:
		return Downloader{}, fmt.Errorf("unsupported network ID: %d. Fuji only supported", network.ID)
	}
}

func (d Downloader) Download() error {
	d.logger.Info("Download started from", zap.String("url", d.getter.request.URL().String()))

	d.currentOp.Lock()
	defer d.currentOp.Unlock()

	resp := d.getter.client.Do(d.getter.request)
	d.setDownloadSize(resp.Size())
	d.logger.Debug("Download response received",
		zap.String("status", resp.HTTPResponse.Status))
	t := time.NewTicker(updateInterval)
	defer t.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-t.C:
				d.setBytesComplete(resp.BytesComplete())
				d.logger.Info("Download progress",
					zap.Int64("bytesComplete", d.GetBytesComplete()),
					zap.Int64("size", d.GetDownloadSize()))
			case <-resp.Done:
				return
			}
		}
	}()
	<-resp.Done // Wait for the download to finish
	t.Stop()    // Stop the ticker
	<-done

	// check for errors
	if err := resp.Err(); err != nil {
		d.logger.Error("Download failed", zap.Error(err))
		return err
	}

	d.logger.Info("Download saved to", zap.String("path", d.getter.request.Filename))
	return nil
}

func (d Downloader) UnpackTo(targetDir string) error {
	d.currentOp.Lock()
	defer d.currentOp.Unlock()
	// prepare destination path
	if err := os.MkdirAll(targetDir, constants.UserOnlyWriteReadExecPerms); err != nil {
		d.logger.Error("Failed to create target directory", zap.Error(err))
		return err
	}
	tarFile, err := os.Open(d.getter.request.Filename)
	if err != nil {
		d.logger.Error("Failed to open tar file", zap.Error(err))
		return fmt.Errorf("failed to open tar file: %w", err)
	}
	defer tarFile.Close()

	tarReader := tar.NewReader(io.LimitReader(tarFile, maxFileSize))
	extractedSize := int64(0)
	for {
		// codeql [security] suppressed reason: This usage is safe in this context.
		header, err := tarReader.Next()
		if err == io.EOF {
			d.logger.Debug("End of archive reached")
			break // End of archive
		}
		if err != nil {
			d.logger.Error("Failed to read tar archive", zap.Error(err))
			return fmt.Errorf("error reading tar archive: %w", err)
		}

		relPath, err := filepath.Rel(targetDir, filepath.Join(targetDir, filepath.Clean(header.Name)))
		if err != nil || strings.HasPrefix(relPath, "..") {
			d.logger.Error("Invalid file path", zap.String("path", header.Name))
			return fmt.Errorf("invalid file path: %s", header.Name)
		}
		targetPath := filepath.Join(targetDir, relPath)

		// security checks
		if extractedSize+header.Size > maxFileSize {
			d.logger.Error("File too large", zap.String("path", header.Name), zap.Int64("size", header.Size))
			return fmt.Errorf("file too large: %s", header.Name)
		}
		if strings.Contains(header.Name, "..") {
			d.logger.Error("Invalid file path", zap.String("path", header.Name))
			return fmt.Errorf("invalid file path: %s", header.Name)
		}
		// end of security checks

		switch header.Typeflag {
		case tar.TypeDir:
			d.logger.Debug("Creating directory", zap.String("path", targetPath))
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				d.logger.Error("Failed to create directory", zap.Error(err))
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			d.logger.Debug("Ensure parent directory exists for ", zap.String("path", targetPath))
			if err := os.MkdirAll(filepath.Dir(targetPath), os.FileMode(0o755)); err != nil {
				d.logger.Error("Failed to create parent directory for file", zap.Error(err))
				return fmt.Errorf("failed to create parent directory for file: %w", err)
			}
			d.logger.Debug("Creating file", zap.String("path", targetPath))
			outFile, err := os.Create(targetPath)
			if err != nil {
				d.logger.Error("Failed to create file", zap.Error(err))
				return fmt.Errorf("failed to create file: %w", err)
			}
			defer outFile.Close()
			copied, err := io.CopyN(outFile, tarReader, header.Size)
			if err != nil {
				d.logger.Error("Failed to write file", zap.Error(err))
				return fmt.Errorf("failed to write file: %w", err)
			}
			if copied < header.Size {
				d.logger.Error("Incomplete file write", zap.String("path", targetPath))
				return fmt.Errorf("incomplete file write for %s", targetPath)
			}
			extractedSize += header.Size
			d.logger.Debug("Written bytes", zap.Int64("bytes", extractedSize))
		default:
			d.logger.Debug("Skipping file", zap.String("path", targetPath))
		}
	}
	d.logger.Info("Download unpacked to", zap.String("path", targetDir))
	return nil
}

// IsEmpty returns true if the Downloader is empty and not initialized
func (d Downloader) IsEmpty() bool {
	return d.getter.client == nil
}

// IsComplete returns true if the download is complete
func (d Downloader) IsComplete() bool {
	return d.GetBytesComplete() == d.GetDownloadSize()
}

func (d Downloader) GetFilePath() (string, error) {
	if !d.IsComplete() {
		return "", fmt.Errorf("download is not completed")
	}
	return d.getter.request.Filename, nil
}

// GetDownloadSize returns the size of the download
func (d Downloader) GetDownloadSize() int64 {
	d.getter.mutex.RLock()
	defer d.getter.mutex.RUnlock()
	return d.getter.size
}

func (d Downloader) setDownloadSize(size int64) {
	d.getter.mutex.Lock()
	defer d.getter.mutex.Unlock()
	d.getter.size = size
}

// GetCurrentProgress returns the current download progress
func (d Downloader) GetBytesComplete() int64 {
	d.getter.mutex.RLock()
	defer d.getter.mutex.RUnlock()
	return d.getter.bytesComplete
}

func (d Downloader) setBytesComplete(progress int64) {
	d.getter.mutex.Lock()
	defer d.getter.mutex.Unlock()
	d.getter.bytesComplete = progress
}

func (d Downloader) CleanUp() {
	_ = os.Remove(d.getter.request.Filename)
}
