// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"

	"go.uber.org/zap"
)

type ConnectionSettings struct {
	NetworkID    uint32
	Genesis      []byte
	Upgrade      []byte
	BootstrapIDs []string
	BootstrapIPs []string
}

func CreateLocalCluster(
	app *application.Avalanche,
	ctx context.Context,
	printFunc func(msg string, args ...interface{}),
	clusterName string,
	avalancheGoBinPath string,
	pluginDir string,
	defaultFlags map[string]interface{},
	connectionSettings ConnectionSettings,
	numNodes uint32,
	nodeSettings []NodeSettings,
	networkKind models.Network,
) (*tmpnet.Network, error) {
	if len(connectionSettings.BootstrapIDs) != len(connectionSettings.BootstrapIPs) {
		return nil, fmt.Errorf("number of bootstrap IDs and bootstrap IP:port pairs must be equal")
	}
	nodes, err := GetNewTmpNetNodes(numNodes, nodeSettings)
	if err != nil {
		return nil, err
	}
	var unparsedGenesis *genesis.UnparsedConfig
	if len(connectionSettings.Genesis) > 0 {
		unparsedGenesis := &genesis.UnparsedConfig{}
		if err := json.Unmarshal(connectionSettings.Genesis, unparsedGenesis); err != nil {
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
		connectionSettings.BootstrapIPs,
		connectionSettings.BootstrapIDs,
		unparsedGenesis,
		connectionSettings.Upgrade,
		defaultFlags,
		nodes,
		false,
	)
	if err != nil {
		return nil, err
	}
	if err := TmpNetEnableSybilProtection(networkDir); err != nil {
		return nil, err
	}
	// preseed nodes db from public archive. ignore errors
	nodeIDs := []string{}
	for _, node := range network.Nodes {
		nodeIDs = append(nodeIDs, node.NodeID.String())
	}
	if err := DownloadAvalancheGoDB(networkKind, networkDir, nodeIDs, app.Log, printFunc); err != nil {
		app.Log.Info("seeding public archive data finished with error: %v. Ignored if any", zap.Error(err))
	}
	if err := TmpNetBootstrap(ctx, app.Log, networkDir); err != nil {
		return nil, err
	}
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

func ClusterIsRunning(app *application.Avalanche, clusterName string) (bool, error) {
	networkDir := GetLocalClusterDir(app, clusterName)
	status, err := GetTmpNetRunningStatus(networkDir)
	if err != nil {
		return false, err
	}
	return status == Running, nil
}

func IsLocalNetworkCluster(app *application.Avalanche, clusterName string) (bool, error) {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return false, err
	}
	return network.GetNetworkID() == constants.LocalNetworkID, nil
}

func GetClusterNetworkKind(app *application.Avalanche, clusterName string) (models.Network, error) {
	networkDir := GetLocalClusterDir(app, clusterName)
	networkID, err := GetTmpNetNetworkID(networkDir)
	if err != nil {
		return models.UndefinedNetwork, err
	}
	return models.NetworkFromNetworkID(networkID), nil
}

func GetLocalNetworkRunningClusters(app *application.Avalanche) ([]string, error) {
	runningClusters, err := GetRunningClusters(app)
	if err != nil {
		return nil, err
	}
	localNetworkRunningClusters := []string{}
	for _, clusterName := range runningClusters {
		if local, err := IsLocalNetworkCluster(app, clusterName); err != nil {
			return nil, err
		} else if !local {
			continue
		}
		localNetworkRunningClusters = append(localNetworkRunningClusters, clusterName)
	}
	return localNetworkRunningClusters, nil
}

func GetRunningClusters(app *application.Avalanche) ([]string, error) {
	clusters, err := GetClusters(app)
	if err != nil {
		return nil, err
	}
	runningClusters := []string{}
	for _, clusterName := range clusters {
		if running, err := ClusterIsRunning(app, clusterName); err != nil {
			return nil, err
		} else if !running {
			continue
		}
		runningClusters = append(runningClusters, clusterName)
	}
	return runningClusters, nil
}

func GetClusters(app *application.Avalanche) ([]string, error) {
	clusters := []string{}
	clustersDir := app.GetLocalClustersDir()
	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read local clusters dir %s: %w", clustersDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		clusterName := entry.Name()
		if _, err := GetLocalCluster(app, clusterName); err != nil {
			return nil, fmt.Errorf("failure loading cluster %s: %w", clusterName, err)
		}
		clusters = append(clusters, clusterName)
	}
	return clusters, nil
}

func WaitLocalClusterBlockchainBootstrapped(
	app *application.Avalanche,
	ctx context.Context,
	clusterName string,
	blockchainID string,
	subnetID ids.ID,
) error {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return err
	}
	return WaitTmpNetBlockchainBootstrapped(ctx, network, blockchainID, subnetID)
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
	connectionSettings.BootstrapIPs, connectionSettings.BootstrapIDs, err = GetTmpNetBootstrappers(networkDir, ids.EmptyNodeID)
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

// Indicates if a blockchain is bootstrapped on the local network
// If the network has no validators for the blockchain, it fails
func IsLocalClusterBlockchainBootstrapped(
	app *application.Avalanche,
	clusterName string,
	blockchainID string,
	subnetID ids.ID,
) (bool, error) {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return false, err
	}
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	return IsTmpNetBlockchainBootstrapped(ctx, network, blockchainID, subnetID)
}

// Indicates if P-Chain is bootstrapped on the network, and also if
// all blockchain that have validators on the network, are bootstrapped
func LocalClusterHealth(
	app *application.Avalanche,
	clusterName string,
) (bool, bool, error) {
	pChainBootstrapped, err := IsLocalClusterBlockchainBootstrapped(app, clusterName, "P", ids.Empty)
	if err != nil {
		return false, false, err
	}
	blockchains, err := GetLocalClusterBlockchainInfo(app, clusterName)
	if err != nil {
		return pChainBootstrapped, false, err
	}
	for _, blockchain := range blockchains {
		if hasValidators, err := LocalClusterHasValidatorsForSubnet(app, clusterName, blockchain.SubnetID); err != nil {
			return pChainBootstrapped, false, err
		} else if !hasValidators {
			continue
		}
		if blockchainBootstrapped, err := IsLocalClusterBlockchainBootstrapped(app, clusterName, blockchain.ID.String(), blockchain.SubnetID); err != nil {
			return pChainBootstrapped, false, err
		} else if !blockchainBootstrapped {
			return pChainBootstrapped, false, nil
		}
	}
	return pChainBootstrapped, true, nil
}

// Returns blockchain info for all non standard blockchains deployed into the network
func GetLocalClusterBlockchainInfo(
	app *application.Avalanche,
	clusterName string,
) ([]BlockchainInfo, error) {
	endpoint, err := GetLocalClusterEndpoint(app, clusterName)
	if err != nil {
		return nil, err
	}
	return GetBlockchainInfo(endpoint)
}

// Returns the endpoint associated to the cluster
// If the network is not alive it errors
func GetLocalClusterEndpoint(
	app *application.Avalanche,
	clusterName string,
) (string, error) {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return "", err
	}
	return GetTmpNetEndpoint(network)
}

// Indicates if the cluster validates a subnet at all
func LocalClusterHasValidatorsForSubnet(
	app *application.Avalanche,
	clusterName string,
	subnetID ids.ID,
) (bool, error) {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return false, err
	}
	return TmpNetHasValidatorsForSubnet(network, subnetID)
}

func GetLocalClusterURIs(
	app *application.Avalanche,
	clusterName string,
) ([]string, error) {
	networkDir := GetLocalClusterDir(app, clusterName)
	return GetTmpNetNodeURIsWithFix(networkDir)
}
