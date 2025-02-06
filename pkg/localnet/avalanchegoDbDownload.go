// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/sdk/network"
	"github.com/ava-labs/avalanche-cli/sdk/publicarchive"
	"github.com/ava-labs/avalanchego/utils/logging"
)

func DownloadAvalancheGoDb(
	clusterNetwork models.Network,
	rootDir string,
	nodeNames []string,
) error {
	// only for fuji
	if clusterNetwork.Kind != models.Fuji {
		return nil
	}
	network := network.FujiNetwork()
	ux.Logger.Info("downloading public archive for network %s", clusterNetwork.Name())
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
		ux.Logger.Info("public archive downloaded to %s", path)
	}

	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	var firstErr error

	for _, nodeName := range nodeNames {
		target := filepath.Join(rootDir, nodeName, "db")
		ux.Logger.Info("unpacking public archive to %s", target)

		// Skip if target already exists
		if _, err := os.Stat(target); err == nil {
			ux.Logger.Info("data folder already exists at %s. Skipping...", target)
			continue
		}
		wg.Add(1)
		go func(target string) {
			defer wg.Done()

			if err := publicArcDownloader.UnpackTo(target); err != nil {
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
	ux.Logger.PrintToUser("Public archive unpacked to: %s", rootDir)
	return nil
}

func cleanUpClusterNodeData(rootDir string, nodesNames []string) error {
	for _, nodeName := range nodesNames {
		if err := os.RemoveAll(filepath.Join(rootDir, nodeName)); err != nil {
			return err
		}
	}
	return nil
}
