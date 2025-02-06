// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"os"
	"context"
	"encoding/json"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
)

type ConnectionSettings struct {
	NetworkID uint32
	Genesis []byte
	Upgrade []byte
	BootstrapIDs         []string
	BootstrapIPs         []string
}

func CreateLocalCluster(
	app *application.Avalanche,
	ctx context.Context,
	clusterName string,
	avalancheGoBinPath string,
	pluginDir string,
	defaultFlags map[string]interface{},
	connectionSettings ConnectionSettings,
	numNodes uint32,
	nodeSettings []NodeSettings,
) (*tmpnet.Network, error) {
	if len(connectionSettings.BootstrapIDs) != len(connectionSettings.BootstrapIPs) {
		return nil, fmt.Errorf("number of bootstrap IDs and bootstrap IP:port pairs must be equal")
	}
	nodes, err := GetNewTmpNetNodes(numNodes, nodeSettings)
	if err != nil {
		return nil, err
	}
        genesis := genesis.UnparsedConfig{}
	if len(connectionSettings.Genesis) > 0 {
		if err := json.Unmarshal(connectionSettings.Genesis, &genesis); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis: %w", err)
		}
	}
	networkDir := GetLocalClusterDir(app, clusterName)
	network, err := TmpNetCreate(
		ctx,
		app.Log,
		networkDir,
		avalancheGoBinPath,
		pluginDir,
		connectionSettings.NetworkID,
		&genesis,
		connectionSettings.Upgrade,
		defaultFlags,
		nodes,
		false,
	)
	if err != nil {
		return network, err
	}
	/*
	if err := network.Bootstrap(ctx, app.Log); err != nil {
		return network, err
	}
	*/
	return network, nil
}

// Returns the directory associated to the local cluster
func GetLocalClusterDir(
	app *application.Avalanche,
	clusterName string,
) string {
	return app.GetLocalClusterDir(clusterName)
}

// Returns the tmpnet associated to the local cluster
func GetLocalCluster(
	app *application.Avalanche,
	clusterName string,
) (*tmpnet.Network, error) {
	networkDir := GetLocalClusterDir(app, clusterName)
	return GetTmpNetNetwork(networkDir)
}

// Indicates if the local cluster exists and has valid data on its directory
func LocalClusterExists(
	app *application.Avalanche,
	clusterName string,
) bool {
	_, err := GetLocalCluster(app, clusterName)
	return err == nil
}

// Stops a local cluster
func LocalClusterStop(
	app *application.Avalanche,
	clusterName string,
) error {
	networkDir := GetLocalClusterDir(app, clusterName)
	return TmpNetStop(networkDir)
}

// Removes a local cluster
func LocalClusterRemove(
	app *application.Avalanche,
	clusterName string,
) error {
	if clusterName == "" {
		return fmt.Errorf("invalid cluster '%s'", clusterName)
	}
	networkDir := GetLocalClusterDir(app, clusterName)
	if !sdkutils.DirExists(networkDir) {
		return fmt.Errorf("cluster directory %s does not exist", networkDir)
	}
	_ = LocalClusterStop(app, clusterName)
	return os.RemoveAll(networkDir)
}

func IsLocalNetworkCluster(clusterName string) (bool, error) {
	return false, fmt.Errorf("unimplemented")
}

func GetLocalNetworkClusters(app *application.Avalanche) ([]string, error) {
	localNetworkClusters := []string{}
	clustersDir := app.GetLocalClustersDir()
	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read local clusters dir %s: %w", clustersDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if b, err := IsLocalNetworkCluster(entry.Name()); err != nil {
			return nil, err
		} else if b {
			localNetworkClusters = append(localNetworkClusters, entry.Name())
		}
	}
	return localNetworkClusters, nil
}

func GetRunningClusters(app *application.Avalanche) ([]string, error) {
	return nil, fmt.Errorf("unimplemented")
}

func WaitLocalClusterBlockchainBootstrapped(
	app *application.Avalanche,
	ctx context.Context,
	clusterName string,
	blockchainID string,
	subnetID ids.ID,
) error {
	networkDir := GetLocalClusterDir(app, clusterName)
	return WaitTmpNetBlockchainBootstrapped(ctx, networkDir, blockchainID, subnetID)
}

func GetLocalNetworkConnectionInfo(
	app *application.Avalanche,
) (ConnectionSettings, error) {
	connectionSettings := ConnectionSettings{}
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return ConnectionSettings{}, fmt.Errorf("failed to connect to local network: %w", err)
	}
	connectionSettings.NetworkID, err = GetTmpNetNetworkID(networkDir)
	if err != nil {
		return ConnectionSettings{}, err
	}
	connectionSettings.BootstrapIPs, connectionSettings.BootstrapIDs, err = GetTmpNetBootstrappers(networkDir)
	if err != nil {
		return ConnectionSettings{}, err
	}
	connectionSettings.Genesis, err = GetTmpNetGenesis(networkDir)
	if err != nil {
		return ConnectionSettings{}, err
	}
	connectionSettings.Upgrade, err = GetTmpNetUpgrade(networkDir)
	if err != nil {
		return ConnectionSettings{}, err
	}
	return connectionSettings, nil
}
