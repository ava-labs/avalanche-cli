// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	avagoconstants "github.com/ava-labs/avalanchego/utils/constants"
)

// GetNetworkFromCluster gets the network that a cluster is on
func GetNetworkFromCluster(clusterConfig models.ClusterConfig) (models.Network, error) {
	network := clusterConfig.Network
	switch {
	case network.ID == constants.LocalNetworkID:
		return models.NewLocalNetwork(), nil
	case network.ID == avagoconstants.FujiID:
		return models.NewFujiNetwork(), nil
	case network.ID == avagoconstants.MainnetID:
		return models.NewMainnetNetwork(), nil
	case network.ID == constants.EtnaDevnetNetworkID:
		return models.NewEtnaDevnetNetwork(), nil
	default:
		return models.UndefinedNetwork, fmt.Errorf("unable to get network from cluster %s", network.ClusterName)
	}
}

func GetHostWithCloudID(app *application.Avalanche, clusterName string, cloudID string) (*models.Host, error) {
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	monitoringInventoryFile := app.GetMonitoringInventoryDir(clusterName)
	if utils.FileExists(monitoringInventoryFile) {
		monitoringHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryFile)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, monitoringHosts...)
	}
	for _, host := range hosts {
		if host.GetCloudID() == cloudID {
			return host, nil
		}
	}
	return nil, nil
}

func GetAWMRelayerHost(app *application.Avalanche, clusterName string) (*models.Host, error) {
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return nil, err
	}
	relayerCloudID := ""
	for _, cloudID := range clusterConfig.GetCloudIDs() {
		if nodeConfig, err := app.LoadClusterNodeConfig(cloudID); err != nil {
			return nil, err
		} else if nodeConfig.IsAWMRelayer {
			relayerCloudID = nodeConfig.NodeID
		}
	}
	return GetHostWithCloudID(app, clusterName, relayerCloudID)
}
