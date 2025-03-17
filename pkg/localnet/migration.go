	// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"path/filepath"
	"os"
	"encoding/json"
	"encoding/base64"
	"strings"

	avagoconfig "github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-network-runner/network"

	dircopy "github.com/otiai10/copy"
)

const migratedSuffix = "-migrated"

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
			if strings.HasSuffix(clusterName, migratedSuffix) {
				printFunc("%s was partially migrated with failure. Please manually recover", networkDir)
				continue
			}
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
		if err := migrateCluster(app, printFunc, clusterName); err != nil {
			printFunc("Failure migrating %s at %s: %s", clusterName, GetLocalClusterDir(app, clusterName), err)
		}
	}
	if clusterToReload != "" {
		printFunc("Restarting cluster %s.", clusterToReload)
	}
	return nil
}

func migrateCluster(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	clusterName string,
) error {
	networkDir := GetLocalClusterDir(app, clusterName)
	anrDir := GetLocalClusterDir(app, clusterName + migratedSuffix)
	if err := os.Rename(networkDir, anrDir); err != nil {
		return err
	}
	jsonPath := filepath.Join(anrDir, "network.json")
	bs, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}
	var config network.Config
	if err := json.Unmarshal(bs, &config); err != nil {
		return err
	}
	connectionSettings := ConnectionSettings{
		NetworkID: config.NetworkID,
	}
	trackSubnetsStr := ""
	nodeSettings := []NodeSettings{}
	for _, nodeConfig := range config.NodeConfigs {
		decodedStakingSigningKey, err := base64.StdEncoding.DecodeString(nodeConfig.StakingSigningKey)
		if err != nil {
			return err
		}
		httpPort, err := utils.GetJSONKey[float64](nodeConfig.Flags, avagoconfig.HTTPPortKey)
		if err != nil {
			return fmt.Errorf("failure reading legacy local network conf: %w", err)
		}
		stakingPort, err := utils.GetJSONKey[float64](nodeConfig.Flags, avagoconfig.StakingPortKey)
		if err != nil {
			return fmt.Errorf("failure reading legacy local network conf: %w", err)
		}
		trackSubnetsStr, err = utils.GetJSONKey[string](nodeConfig.Flags, avagoconfig.TrackSubnetsKey)
		if err != nil {
			return fmt.Errorf("failure reading legacy local network conf: %w", err)
		}
		nodeSettings = append(nodeSettings, NodeSettings{
			StakingTLSKey: []byte(nodeConfig.StakingKey),
			StakingCertKey: []byte(nodeConfig.StakingCert),
			StakingSignerKey: decodedStakingSigningKey,
			HTTPPort: uint64(httpPort),
			P2PPort: uint64(stakingPort),
		})
	}
	trackedSubnets, err := utils.MapWithError(strings.Split(trackSubnetsStr, ","), func (s string) (ids.ID, error){return ids.FromString(s)})
	if err != nil {
		return err
	}
	binPath := config.BinaryPath
	networkModel := models.NetworkFromNetworkID(connectionSettings.NetworkID)
	//
	pluginDir := filepath.Join(networkDir, "plugins")
	if err := os.MkdirAll(networkDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create network directory %s: %w", networkDir, err)
	}
	if err := os.MkdirAll(pluginDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create plugin directory %s: %w", pluginDir, err)
	}
	// defaultFlags
	defaultFlags := map[string]interface{}{}
	defaultFlags[avagoconfig.PartialSyncPrimaryNetworkKey] = true
	defaultFlags[avagoconfig.NetworkAllowPrivateIPsKey] = true
	defaultFlags[avagoconfig.IndexEnabledKey] = false
	defaultFlags[avagoconfig.IndexAllowIncompleteKey] = true
	network, err := CreateLocalCluster(
		app,
		printFunc,
		clusterName,
		binPath,
		pluginDir,
		defaultFlags,
		connectionSettings,
		uint32(len(nodeSettings)),
		nodeSettings,
		trackedSubnets,
		networkModel,
		false,
		false,
	)
	if err != nil {
		return err
	}
	for i, node := range network.Nodes {
		sourceDir := filepath.Join(anrDir, config.NodeConfigs[i].Name, "db")
		targetDir := filepath.Join(networkDir, node.NodeID.String(), "db")
		if err := dircopy.Copy(sourceDir, targetDir); err != nil {
			return fmt.Errorf("failure migrating data dir %s into %s: %w", sourceDir, targetDir, err)
		}
		sourceDir = filepath.Join(anrDir, config.NodeConfigs[i].Name, "plugins")
		targetDir = filepath.Join(networkDir, "plugins")
		if err := dircopy.Copy(sourceDir, targetDir); err != nil {
			return fmt.Errorf("failure migrating plugindir dir %s into %s: %w", sourceDir, targetDir, err)
		}
	}
	return nil
}
