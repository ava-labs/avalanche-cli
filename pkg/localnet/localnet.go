// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"

	"golang.org/x/exp/maps"
)

var ErrNetworkNotRunning = errors.New("network is not running")

// Indicates if all, some or none of the local network nodes are running
func LocalNetworkRunningStatus(app *application.Avalanche) (RunningStatus, error) {
	if LocalNetworkMetaExists(app) {
		meta, err := GetLocalNetworkMeta(app)
		if err != nil {
			return UndefinedRunningStatus, err
		}
		if sdkutils.DirExists(meta.NetworkDir) {
			status, err := GetTmpNetRunningStatus(meta.NetworkDir)
			if err != nil {
				return status, err
			}
			if status == NotRunning {
				if err := RemoveLocalNetworkMeta(app); err != nil {
					return NotRunning, err
				}
			}
			return status, nil
		}
	}
	return NotRunning, nil
}

// Returns true if all local network nodes are running
func IsLocalNetworkRunning(app *application.Avalanche) (bool, error) {
	status, err := LocalNetworkRunningStatus(app)
	if err != nil {
		return false, err
	}
	return status == Running, nil
}

// Returns the tmpnet directory associated to the local network
// If the network is not alive it errors
func GetLocalNetworkDir(app *application.Avalanche) (string, error) {
	isRunning, err := IsLocalNetworkRunning(app)
	if err != nil {
		return "", err
	}
	if !isRunning {
		return "", ErrNetworkNotRunning
	}
	meta, err := GetLocalNetworkMeta(app)
	if err != nil {
		return "", err
	}
	return meta.NetworkDir, nil
}

// Returns the tmpnet associated to the local network
// If the network is not alive it errors
func GetLocalNetwork(app *application.Avalanche) (*tmpnet.Network, error) {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return nil, err
	}
	return GetTmpNetNetworkWithLog(app.Log, networkDir)
}

// Returns the endpoint associated to the local network
// If the network is not alive it errors
func GetLocalNetworkEndpoint(app *application.Avalanche) (string, error) {
	network, err := GetLocalNetwork(app)
	if err != nil {
		return "", err
	}
	return GetTmpNetEndpoint(network)
}

// Returns blockchain info for all non standard blockchains deployed into the local network
func GetLocalNetworkBlockchainsInfo(app *application.Avalanche) ([]BlockchainInfo, error) {
	endpoint, err := GetLocalNetworkEndpoint(app)
	if err != nil {
		return nil, err
	}
	return GetBlockchainsInfo(endpoint)
}

// Returns avalanchego version and RPC version for the local network
func GetLocalNetworkAvalancheGoVersion(app *application.Avalanche) (bool, string, int, error) {
	// not actually an error, network just not running
	if isRunning, err := IsLocalNetworkRunning(app); err != nil {
		return true, "", 0, err
	} else if !isRunning {
		return false, "", 0, nil
	}
	endpoint, err := GetLocalNetworkEndpoint(app)
	if err != nil {
		return true, "", 0, err
	}
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	infoClient := info.NewClient(endpoint)
	versionResponse, err := infoClient.GetNodeVersion(ctx)
	if err != nil {
		return true, "", 0, err
	}
	// version is in format avalanche/x.y.z, need to turn to semantic
	splitVersion := strings.Split(versionResponse.Version, "/")
	if len(splitVersion) != 2 {
		return true, "", 0, fmt.Errorf("%s", "unable to parse avalanchego version "+versionResponse.Version)
	}
	// index 0 should be avalanche, index 1 will be version
	parsedVersion := "v" + splitVersion[1]
	return true, parsedVersion, int(versionResponse.RPCProtocolVersion), nil
}

// Stops the local network
func LocalNetworkStop(app *application.Avalanche) error {
	networkDir, err := GetLocalNetworkDir(app)
	if err != nil {
		return err
	}
	if err := TmpNetStop(networkDir); err != nil {
		return err
	}
	return RemoveLocalNetworkMeta(app)
}

// Returns a context large enough to support all local network operations
func GetLocalNetworkDefaultContext() (context.Context, context.CancelFunc) {
	return sdkutils.GetTimedContext(constants.LocalBootstrapTimeout)
}

// Indicates if the local network tracks a subnet at all
func IsLocalNetworkTrackingSubnet(
	app *application.Avalanche,
	subnetID ids.ID,
) (bool, error) {
	network, err := GetLocalNetwork(app)
	if err != nil {
		return false, err
	}
	return IsTmpNetNodeTrackingSubnet(network.Nodes, subnetID)
}

// Indicates if a blockchain is bootstrapped on the local network
// If the network has no validators for the blockchain, it fails
func IsLocalNetworkBlockchainBootstrapped(
	app *application.Avalanche,
	blockchainID string,
	subnetID ids.ID,
) (bool, error) {
	network, err := GetLocalNetwork(app)
	if err != nil {
		return false, err
	}
	ctx, cancel := sdkutils.GetAPIContext()
	defer cancel()
	return IsTmpNetBlockchainBootstrapped(ctx, network, blockchainID, subnetID)
}

// Indicates if P-Chain is bootstrapped on the local network, and also if
// all blockchains that have validators on the local network, or in clusters
// connected to the local network, are bootstrapped
func LocalNetworkHealth(
	app *application.Avalanche,
) (bool, bool, error) {
	pChainBootstrapped, err := IsLocalNetworkBlockchainBootstrapped(app, "P", ids.Empty)
	if err != nil {
		return false, false, err
	}
	blockchains, err := GetLocalNetworkBlockchainsInfo(app)
	if err != nil {
		return pChainBootstrapped, false, err
	}
	clusters, err := GetRunningLocalClustersConnectedToLocalNetwork(app)
	if err != nil {
		return pChainBootstrapped, false, err
	}
	for _, blockchain := range blockchains {
		isTracking, err := IsLocalNetworkTrackingSubnet(app, blockchain.SubnetID)
		if err != nil {
			return pChainBootstrapped, false, err
		}
		if !isTracking {
			blockchainBootstrappedOnSomeCluster := false
			for _, clusterName := range clusters {
				if isTracking, err := IsLocalClusterTrackingSubnet(app, clusterName, blockchain.SubnetID); err != nil {
					return pChainBootstrapped, false, err
				} else if !isTracking {
					continue
				}
				blockchainBootstrappedOnSomeCluster, err = IsLocalClusterBlockchainBootstrapped(app, clusterName, blockchain.ID.String(), blockchain.SubnetID)
				if err != nil {
					return pChainBootstrapped, false, err
				}
				if blockchainBootstrappedOnSomeCluster {
					break
				}
			}
			if !blockchainBootstrappedOnSomeCluster {
				return pChainBootstrapped, false, nil
			}
		} else {
			blockchainBootstrapped, err := IsLocalNetworkBlockchainBootstrapped(app, blockchain.ID.String(), blockchain.SubnetID)
			if err != nil {
				return pChainBootstrapped, false, err
			}
			if !blockchainBootstrapped {
				return pChainBootstrapped, false, nil
			}
		}
	}
	return pChainBootstrapped, true, nil
}

// Create a local network of [numNodes] nodes at [networkDir] using avalanchego binary at [avalancheGoBinPath]
// Make local network meta to point to it
func CreateLocalNetwork(
	app *application.Avalanche,
	networkDir string,
	numNodes uint32,
	pluginDir string,
	avalancheGoBinPath string,
	cChainConfig []byte,
) error {
	// get default network conf for NumNodes
	networkID, unparsedGenesis, upgradeBytes, defaultFlags, nodes, err := GetDefaultNetworkConf(numNodes)
	if err != nil {
		return err
	}
	// add node flags on CLI config info default network flags
	nodeConfigStr, err := app.Conf.LoadNodeConfig()
	if err != nil {
		return err
	}
	var nodeConfig map[string]interface{}
	if err := json.Unmarshal([]byte(nodeConfigStr), &nodeConfig); err != nil {
		return fmt.Errorf("invalid common node config JSON: %w", err)
	}
	maps.Copy(defaultFlags, nodeConfig)
	// create network
	ctx, cancel := GetLocalNetworkDefaultContext()
	defer cancel()
	if _, err := TmpNetCreate(
		ctx,
		app.Log,
		networkDir,
		avalancheGoBinPath,
		cChainConfig,
		pluginDir,
		networkID,
		nil,
		nil,
		unparsedGenesis,
		upgradeBytes,
		defaultFlags,
		nodes,
		true,
	); err != nil {
		_ = TmpNetStop(networkDir)
		return err
	}
	// save network directory
	return SaveLocalNetworkMeta(app, networkDir)
}

// Load a local network at [networkDir] using avalanchego binary at [avalancheGoBinPath]
// Make local network meta to point to it
func LoadLocalNetwork(
	app *application.Avalanche,
	networkDir string,
	avalancheGoBinPath string,
) error {
	// add node flags on CLI config info flags
	nodeConfigStr, err := app.Conf.LoadNodeConfig()
	if err != nil {
		return err
	}
	var nodeConfig map[string]interface{}
	if err := json.Unmarshal([]byte(nodeConfigStr), &nodeConfig); err != nil {
		return fmt.Errorf("invalid common node config JSON: %w", err)
	}
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	for i := range network.Nodes {
		for k, v := range nodeConfig {
			sv, ok := v.(string)
			if ok {
				network.Nodes[i].Flags[k] = sv
			}
		}
	}
	if err := network.Write(); err != nil {
		return err
	}
	// local network
	ctx, cancel := GetLocalNetworkDefaultContext()
	defer cancel()
	if _, err := TmpNetLoad(ctx, app.Log, networkDir, avalancheGoBinPath); err != nil {
		_ = TmpNetStop(networkDir)
		return err
	}
	// set aliases
	if err := TmpNetSetDefaultAliases(ctx, networkDir); err != nil {
		return err
	}
	// save network directory
	return SaveLocalNetworkMeta(app, networkDir)
}
