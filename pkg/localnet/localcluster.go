// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"

	dircopy "github.com/otiai10/copy"
	"go.uber.org/zap"
)

// A connection setting is either the network ID for a public network,
// or the full settings for a custom network
type ConnectionSettings struct {
	NetworkID    uint32
	Genesis      []byte
	Upgrade      []byte
	BootstrapIDs []string
	BootstrapIPs []string
}

// Create a local cluster [clusterName] connected to another network,
// based on [connectionSettings].
// Set up [numNodes] nodes, either with fresh keys and ports, or based on settings given by [nodeSettings]
// If [downloadDB] is set, and network is fuji, downloads the current avalanchego DB -note: db download is not desired
// if migrating from a network runner cluster-
// If [bootstrap] is set, starts the nodes
func CreateLocalCluster(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	clusterName string,
	avalancheGoBinPath string,
	cChainConfig []byte,
	pluginDir string,
	defaultFlags map[string]interface{},
	connectionSettings ConnectionSettings,
	numNodes uint32,
	nodeSettings []NodeSetting,
	trackedSubnets []ids.ID,
	networkModel models.Network,
	downloadDB bool,
	bootstrap bool,
) (*tmpnet.Network, error) {
	if len(connectionSettings.BootstrapIDs) != len(connectionSettings.BootstrapIPs) {
		return nil, fmt.Errorf("number of bootstrap IDs and bootstrap IP:port pairs must be equal")
	}
	nodes, err := GetNewTmpNetNodes(numNodes, nodeSettings, trackedSubnets)
	if err != nil {
		return nil, err
	}
	var unparsedGenesis *genesis.UnparsedConfig
	if len(connectionSettings.Genesis) > 0 {
		unparsedGenesis = &genesis.UnparsedConfig{}
		if err := json.Unmarshal(connectionSettings.Genesis, unparsedGenesis); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis: %w", err)
		}
	}
	ctx, cancel := networkModel.BootstrappingContext()
	defer cancel()
	networkDir := GetLocalClusterDir(app, clusterName)
	network, err := TmpNetCreate(
		ctx,
		app.Log,
		networkDir,
		avalancheGoBinPath,
		cChainConfig,
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
	// for 1-node clusters we need to overwrite tmpnet's default
	if err := TmpNetEnableSybilProtection(networkDir); err != nil {
		return nil, err
	}
	if downloadDB {
		// preseed nodes db from public archive. ignore errors
		nodeIDs := []string{}
		for _, node := range network.Nodes {
			nodeIDs = append(nodeIDs, node.NodeID.String())
		}
		if err := DownloadAvalancheGoDB(networkModel, networkDir, nodeIDs, app.Log, printFunc); err != nil {
			app.Log.Info("seeding public archive data finished with error: %v. Ignored if any", zap.Error(err))
		}
	}
	if bootstrap {
		if err := TmpNetBootstrap(ctx, app.Log, networkDir); err != nil {
			return nil, err
		}
	}
	return network, nil
}

// Adds a new fresh node with given [httpPort] and [stakingPort]
// into cluster [clusterName] conf, and starts it
// Copies all node conf from the first node of the cluster,
// including connection settings, tracked subnets, blockchain config files.
// Downloads avalanchego DB for fuji nodes
// Finally waits for all the blockchains validated by the cluster to be bootstrapped
func AddNodeToLocalCluster(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	clusterName string,
	httpPort uint32,
	stakingPort uint32,
) (*tmpnet.Node, error) {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return nil, err
	}
	node, err := GetTmpNetFirstNode(network)
	if err != nil {
		return nil, err
	}
	// copy network connection info + tracked subnets
	// creates node dir
	newNode, err := TmpNetCopyNode(node)
	if err != nil {
		return nil, err
	}
	// copy chain config files into new dir
	networkDir := GetLocalClusterDir(app, clusterName)
	sourceDir := filepath.Join(networkDir, node.NodeID.String(), "configs", "chains")
	targetDir := filepath.Join(networkDir, newNode.NodeID.String(), "configs", "chains")
	if err := dircopy.Copy(sourceDir, targetDir); err != nil {
		return nil, fmt.Errorf("failure migrating chain configs dir %s into %s: %w", sourceDir, targetDir, err)
	}
	nodeIDs := []string{newNode.NodeID.String()}
	networkModel, err := GetLocalClusterNetworkModel(app, clusterName)
	if err != nil {
		return nil, err
	}
	if err := DownloadAvalancheGoDB(networkModel, networkDir, nodeIDs, app.Log, printFunc); err != nil {
		app.Log.Info("seeding public archive data finished with error: %v. Ignored if any", zap.Error(err))
	}
	printFunc("Waiting for node: %s to be bootstrapping P-Chain", newNode.NodeID)
	ctx, cancel := networkModel.BootstrappingContext()
	defer cancel()
	if err = TmpNetAddNode(
		ctx,
		app.Log,
		network,
		newNode,
		httpPort,
		stakingPort,
	); err != nil {
		return nil, err
	}
	blockchains, err := GetLocalClusterTrackedBlockchains(app, clusterName)
	if err != nil {
		return nil, err
	}
	for _, blockchain := range blockchains {
		printFunc("Waiting for node: %s to be bootstrapping %s", newNode.NodeID, blockchain.Name)
		if err := WaitLocalClusterBlockchainBootstrapped(
			ctx,
			app,
			clusterName,
			blockchain.ID.String(),
			blockchain.SubnetID,
		); err != nil {
			return nil, err
		}
	}

	printFunc("")
	printFunc("Node logs directory: %s/%s/logs", networkDir, newNode.NodeID)
	printFunc("")

	printFunc("URI: %s", newNode.URI)
	printFunc("Node ID: %s", newNode.NodeID)
	printFunc("")

	return newNode, nil
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

// Stops local cluster [clusterName]
func LocalClusterStop(
	app *application.Avalanche,
	clusterName string,
) error {
	networkDir := GetLocalClusterDir(app, clusterName)
	return TmpNetStop(networkDir)
}

// Removes local cluster [clusterName]
// First stops it if needed
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

// Indicates if local cluster is running
func LocalClusterIsRunning(app *application.Avalanche, clusterName string) (bool, error) {
	networkDir := GetLocalClusterDir(app, clusterName)
	status, err := GetTmpNetRunningStatus(networkDir)
	if err != nil {
		return false, err
	}
	return status == Running, nil
}

// Indicates if local cluster is partially running (only some nodes are executing)
// Useful for stop and destroy flows that need to accomplish the operation
// regardless the cluster is operative
func LocalClusterIsPartiallyRunning(app *application.Avalanche, clusterName string) (bool, error) {
	networkDir := GetLocalClusterDir(app, clusterName)
	status, err := GetTmpNetRunningStatus(networkDir)
	if err != nil {
		return false, err
	}
	return status != NotRunning, nil
}

// Indicates if the cluster [clusterName] is connected to a network of kind [networkModel]
func LocalClusterIsConnectedToNetwork(
	app *application.Avalanche,
	clusterName string,
	networkModel models.Network,
) (bool, error) {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return false, err
	}
	networkID, err := GetTmpNetNetworkID(network)
	if err != nil {
		return false, err
	}
	return networkID == networkModel.ID, nil
}

// Returns the network model the local cluster given by [clusterName]
func GetLocalClusterNetworkModel(
	app *application.Avalanche,
	clusterName string,
) (models.Network, error) {
	networkDir := GetLocalClusterDir(app, clusterName)
	return GetNetworkModel(networkDir)
}

// Gets a list of clusters connected to local network that are also running
func GetRunningLocalClustersConnectedToLocalNetwork(app *application.Avalanche) ([]string, error) {
	return GetFilteredLocalClusters(app, true, models.NewLocalNetwork(), "")
}

// Gets a list of clusters that are running
func GetRunningLocalClusters(app *application.Avalanche) ([]string, error) {
	return GetFilteredLocalClusters(app, true, models.UndefinedNetwork, "")
}

// Gets a list of clusters filtered by running status, network model, and
// validated blockchains
func GetFilteredLocalClusters(
	app *application.Avalanche,
	running bool,
	network models.Network,
	blockchainName string,
) ([]string, error) {
	clusters, err := GetLocalClusters(app)
	if err != nil {
		return nil, err
	}
	filteredClusters := []string{}
	for _, clusterName := range clusters {
		if running {
			if isRunning, err := LocalClusterIsRunning(app, clusterName); err != nil {
				return nil, err
			} else if !isRunning {
				continue
			}
			if blockchainName != "" {
				blockchains, err := GetLocalClusterTrackedBlockchains(app, clusterName)
				if err != nil {
					return nil, err
				}
				blockchainNames := sdkutils.Map(blockchains, func(i BlockchainInfo) string { return i.Name })
				if !sdkutils.Belongs(blockchainNames, blockchainName) {
					continue
				}
			}
		}
		if network != models.UndefinedNetwork {
			if isForNetwork, err := LocalClusterIsConnectedToNetwork(app, clusterName, network); err != nil {
				return nil, err
			} else if !isForNetwork {
				continue
			}
		}
		filteredClusters = append(filteredClusters, clusterName)
	}
	return filteredClusters, nil
}

// Get list of all local clusters
func GetLocalClusters(app *application.Avalanche) ([]string, error) {
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
			continue
		}
		clusters = append(clusters, clusterName)
	}
	return clusters, nil
}

// Waits for cluster [clusterName] to have [blockchainID] bootstrapped
func WaitLocalClusterBlockchainBootstrapped(
	ctx context.Context,
	app *application.Avalanche,
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

// Get connections settings needed to connect a cluster to the local network
func GetLocalNetworkConnectionInfo(
	app *application.Avalanche,
) (ConnectionSettings, error) {
	connectionSettings := ConnectionSettings{}
	network, err := GetLocalNetwork(app)
	if err != nil {
		return ConnectionSettings{}, fmt.Errorf("failed to connect to local network: %w", err)
	}
	connectionSettings.NetworkID, err = GetTmpNetNetworkID(network)
	if err != nil {
		return ConnectionSettings{}, err
	}
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return ConnectionSettings{}, fmt.Errorf("failed to connect to local network: %w", err)
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
	blockchains, err := GetLocalClusterBlockchainsInfo(app, clusterName)
	if err != nil {
		return pChainBootstrapped, false, err
	}
	for _, blockchain := range blockchains {
		if isTracking, err := IsLocalClusterTrackingSubnet(app, clusterName, blockchain.SubnetID); err != nil {
			return pChainBootstrapped, false, err
		} else if !isTracking {
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
func GetLocalClusterBlockchainsInfo(
	app *application.Avalanche,
	clusterName string,
) ([]BlockchainInfo, error) {
	endpoint, err := GetLocalClusterEndpoint(app, clusterName)
	if err != nil {
		return nil, err
	}
	return GetBlockchainsInfo(endpoint)
}

// Returns blockchain info for all blockchains deployed into the network, that are managed by CLI
func GetLocalClusterManagedBlockchainsInfo(
	app *application.Avalanche,
	clusterName string,
) ([]BlockchainInfo, error) {
	networkModel, err := GetLocalClusterNetworkModel(app, clusterName)
	if err != nil {
		return nil, err
	}
	return GetManagedBlockchainsInfo(app, networkModel)
}

// Returns the endpoint associated to the cluster
// If the network is not running it errors
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

// Indicates if the cluster tracks a subnet at all
func IsLocalClusterTrackingSubnet(
	app *application.Avalanche,
	clusterName string,
	subnetID ids.ID,
) (bool, error) {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return false, err
	}
	return IsTmpNetNodeTrackingSubnet(network.Nodes, subnetID)
}

// Returns the subnets tracked by [clusterName]
func GetLocalClusterTrackedSubnets(
	app *application.Avalanche,
	clusterName string,
) ([]ids.ID, error) {
	network, err := GetLocalCluster(app, clusterName)
	if err != nil {
		return nil, err
	}
	return GetTmpNetTrackedSubnets(network.Nodes)
}

// Get local cluster URIs
func GetLocalClusterURIs(
	app *application.Avalanche,
	clusterName string,
) ([]string, error) {
	networkDir := GetLocalClusterDir(app, clusterName)
	return GetTmpNetNodeURIsWithFix(networkDir)
}

// Return a list of blockchains that are tracked at least by one node in the cluster
func GetLocalClusterTrackedBlockchains(
	app *application.Avalanche,
	clusterName string,
) ([]BlockchainInfo, error) {
	blockchains, err := GetLocalClusterBlockchainsInfo(app, clusterName)
	if err != nil {
		return nil, err
	}
	trackedBlockchains := []BlockchainInfo{}
	for _, blockchain := range blockchains {
		if isTracking, err := IsLocalClusterTrackingSubnet(app, clusterName, blockchain.SubnetID); err != nil {
			return nil, err
		} else if isTracking {
			trackedBlockchains = append(trackedBlockchains, blockchain)
		}
	}
	return trackedBlockchains, nil
}

// Return a list of managed blockchains that are tracked at least by one node in the cluster
func GetLocalClusterManagedTrackedBlockchains(
	app *application.Avalanche,
	clusterName string,
) ([]BlockchainInfo, error) {
	blockchains, err := GetLocalClusterManagedBlockchainsInfo(app, clusterName)
	if err != nil {
		return nil, err
	}
	trackedBlockchains := []BlockchainInfo{}
	for _, blockchain := range blockchains {
		if isTracking, err := IsLocalClusterTrackingSubnet(app, clusterName, blockchain.SubnetID); err != nil {
			return nil, err
		} else if isTracking {
			trackedBlockchains = append(trackedBlockchains, blockchain)
		}
	}
	return trackedBlockchains, nil
}

// Tracks the subnet of [blockchainName] in the cluster given by [clusterName]
func LocalClusterTrackSubnet(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	clusterName string,
	blockchainName string,
) error {
	if !LocalClusterExists(app, clusterName) {
		return fmt.Errorf("local cluster %q is not found", clusterName)
	}
	networkDir := GetLocalClusterDir(app, clusterName)
	return TrackSubnet(
		app,
		printFunc,
		blockchainName,
		networkDir,
		nil,
	)
}

// Loads an already existing cluster [clusterName]
// Waits for all blockchains validated by the cluster to be bootstrapped
// Sets default aliases for all blockchains validated by the cluster
// If [blockchainName] is given, updates the blockchain configuration for it
func LoadLocalCluster(
	app *application.Avalanche,
	clusterName string,
	avalancheGoBinaryPath string,
) error {
	if !LocalClusterExists(app, clusterName) {
		return fmt.Errorf("local cluster %q is not found", clusterName)
	}
	networkDir := GetLocalClusterDir(app, clusterName)
	blockchains, err := GetLocalClusterManagedTrackedBlockchains(app, clusterName)
	if err != nil {
		return err
	}
	blockchainNames := sdkutils.Map(blockchains, func(i BlockchainInfo) string { return i.Name })
	for _, blockchainName := range blockchainNames {
		if err := UpdateBlockchainConfig(
			app,
			networkDir,
			blockchainName,
		); err != nil {
			return err
		}
	}
	networkModel, err := GetLocalClusterNetworkModel(app, clusterName)
	if err != nil {
		return err
	}
	ctx, cancel := networkModel.BootstrappingContext()
	defer cancel()
	if _, err := TmpNetLoad(ctx, app.Log, networkDir, avalancheGoBinaryPath); err != nil {
		return err
	}
	blockchains, err = GetLocalClusterTrackedBlockchains(app, clusterName)
	if err != nil {
		return err
	}
	for _, blockchain := range blockchains {
		if err := WaitLocalClusterBlockchainBootstrapped(
			ctx,
			app,
			clusterName,
			blockchain.ID.String(),
			blockchain.SubnetID,
		); err != nil {
			return err
		}
	}
	if networkModel.Kind == models.Local {
		return TmpNetSetDefaultAliases(ctx, networkDir)
	}
	return nil
}

// Sets default aliases for all blockchains validated by the cluster
func RefreshLocalClusterAliases(
	app *application.Avalanche,
	clusterName string,
) error {
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	networkDir := GetLocalClusterDir(app, clusterName)
	return TmpNetSetDefaultAliases(ctx, networkDir)
}

// Returns stardard cluster name as generated from [network] and [blockchainName]
func LocalClusterName(network models.Network, blockchainName string) string {
	blockchainNameComponent := strings.ReplaceAll(blockchainName, " ", "-")
	networkNameComponent := strings.ReplaceAll(strings.ToLower(network.Name()), " ", "-")
	return fmt.Sprintf("%s-local-node-%s", blockchainNameComponent, networkNameComponent)
}
