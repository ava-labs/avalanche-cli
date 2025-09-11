// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-tooling-sdk-go/network"
	"github.com/ava-labs/avalanche-tooling-sdk-go/publicarchive"
	"github.com/ava-labs/avalanchego/utils/logging"

	"go.uber.org/zap"
)

// Downloads avalanchego database into the given [nodeNames]
// To be used on [fuji] only, after creating the nodes, but previously starting them.
func DownloadAvalancheGoDB(
	app *application.Avalanche,
	clusterNetwork models.Network,
	rootDir string,
	nodeNames []string,
	log logging.Logger,
	printFunc func(msg string, args ...interface{}),
) error {
	// only for fuji
	if clusterNetwork.Kind != models.Fuji {
		return nil
	}
	network := network.FujiNetwork()
	printFunc("Downloading public archive for network %s", clusterNetwork.Name())

	// Check cache first
	dbCacheDir := app.GetDBCacheDir()
	cachedFilePath := filepath.Join(dbCacheDir, clusterNetwork.Name())

	var sourcePath string
	useCache := false

	// Check if cached file exists and is fresh (less than 7 days old)
	if fileInfo, err := os.Stat(cachedFilePath); err == nil {
		fileAge := time.Since(fileInfo.ModTime())
		if fileAge < 7*24*time.Hour {
			log.Info("using cached public archive", zap.String("path", cachedFilePath), zap.Duration("age", fileAge))
			printFunc("Using cached public archive (age: %v)", fileAge.Round(time.Hour))
			sourcePath = cachedFilePath
			useCache = true
		} else {
			log.Info("cached file is too old, downloading fresh copy", zap.Duration("age", fileAge))
			printFunc("Cached file is too old (age: %v), downloading fresh copy", fileAge.Round(time.Hour))
		}
	} else {
		log.Info("no cached file found, downloading fresh copy")
	}

	// If not using cache, download fresh copy
	if !useCache {
		publicArcDownloader, err := publicarchive.NewDownloader(network, logging.NewLogger("public-archive-downloader", logging.NewWrappedCore(logging.Off, os.Stdout, logging.JSON.ConsoleEncoder()))) // off as we run inside of the spinner
		if err != nil {
			return fmt.Errorf("failed to create public archive downloader for network %s: %w", clusterNetwork.Name(), err)
		}

		if err := publicArcDownloader.Download(); err != nil {
			return fmt.Errorf("failed to download public archive: %w", err)
		}
		defer publicArcDownloader.CleanUp()

		if path, err := publicArcDownloader.GetFilePath(); err != nil {
			return fmt.Errorf("failed to get downloaded file path: %w", err)
		} else {
			log.Info("public archive downloaded into", zap.String("path", path))
			sourcePath = path

			// Copy to cache for future use
			// Ensure cache directory exists
			if err := os.MkdirAll(filepath.Dir(cachedFilePath), 0o755); err != nil {
				log.Warn("failed to create cache directory", zap.Error(err))
			} else if err := utils.FileCopy(path, cachedFilePath); err != nil {
				log.Warn("failed to cache downloaded file", zap.Error(err))
				// Continue anyway, this is not a critical error
			} else {
				log.Info("successfully cached public archive", zap.String("cache_path", cachedFilePath))
			}
		}
	}

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	var firstErr error

	for _, nodeName := range nodeNames {
		target := filepath.Join(rootDir, nodeName, "db")
		log.Info("unpacking public archive into", zap.String("target", target))

		// Skip if target already exists
		if _, err := os.Stat(target); err == nil {
			log.Info("data folder already exists. Skipping...", zap.String("target", target))
			continue
		}
		wg.Add(1)
		go func(target string) {
			defer wg.Done()

			if err := binutils.ExtractTarGzFile(sourcePath, target); err != nil {
				// Capture the first error encountered
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to unpack public archive: %w", err)
					_ = cleanUpClusterNodeData(rootDir, nodeNames)
				}
				mu.Unlock()
			}
		}(target)
	}
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	printFunc("Public archive unpacked to: %s", rootDir)
	return nil
}

func cleanUpClusterNodeData(rootDir string, nodesNames []string) error {
	for _, nodeName := range nodesNames {
		if err := os.RemoveAll(filepath.Join(rootDir, nodeName, "db")); err != nil {
			return err
		}
	}
	return nil
}
