// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"path/filepath"
	"os"
	"encoding/json"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-network-runner/network"
)

func MigrateANRToTmpNet(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
) error {
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	clusterToReload := ""
	cli, _ := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if cli != nil {
		// ANR is running
		status, _ := cli.Status(ctx)
		if status != nil && status.ClusterInfo != nil {
			// there is a local cluster up
			if status.ClusterInfo.NetworkId != constants.LocalNetworkID {
				clusterToReload = filepath.Base(status.ClusterInfo.RootDataDir)
				printFunc("Found running cluster %s. Will restart after migration.", clusterToReload)
			}
			if _, err := cli.Stop(ctx); err != nil {
				return fmt.Errorf("failed to stop avalanchego: %w", err)
			}
		}
		if err := binutils.KillgRPCServerProcess(
			app,
			binutils.LocalClusterGRPCServerEndpoint,
			constants.ServerRunFileLocalClusterPrefix,
		); err != nil {
			return err
		}
	}
	clusterToReload = "pp1-local-node-fuji"

	toMigrate := []string{}
	clustersDir := app.GetLocalClustersDir()
	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return fmt.Errorf("failed to read local clusters dir %s: %w", clustersDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		clusterName := entry.Name()
		if _, err := GetLocalCluster(app, clusterName); err != nil {
			// not tmpnet, or dir with failures
			networkDir := filepath.Join(clustersDir, clusterName)
			jsonPath := filepath.Join(networkDir, "network.json")
			if utils.FileExists(jsonPath) {
				bs, err := os.ReadFile(jsonPath)
				if err != nil {
					printFunc("Failure loading JSON on cluster at %s: %s. Please manually recover", networkDir, err)
					continue
				}
				var config network.Config
				if err := json.Unmarshal(bs, &config); err != nil {
					printFunc("Unexpected JSON format on cluster at %s: %s. Please manually recover", networkDir, err)
					continue
				}
				if config.NetworkID == constants.LocalNetworkID {
					printFunc("Found legacy local network cluster at %s. Please manually remove", networkDir, err)
					continue
				}
				toMigrate = append(toMigrate, clusterName)
			} else {
				printFunc("Unexpected format on cluster at %s. Please manually recover", networkDir)
			}
		}
	}

	for _, clusterName := range toMigrate {
		printFunc("Migrating %s", clusterName)
		networkDir := filepath.Join(clustersDir, clusterName)
		jsonPath := filepath.Join(networkDir, "network.json")
		bs, err := os.ReadFile(jsonPath)
		if err != nil {
			return err
		}
		var config network.Config
		if err := json.Unmarshal(bs, &config); err != nil {
			return err
		}
		fmt.Println(config.NetworkID)
	}

	if clusterToReload != "" {
		printFunc("Restarting cluster %s.", clusterToReload)
	}
	return nil
}

