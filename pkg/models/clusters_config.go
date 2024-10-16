// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	"golang.org/x/exp/slices"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

type GCPConfig struct {
	ProjectName        string // name of GCP Project
	ServiceAccFilePath string // location of GCP service account key file path
}

type ExtraNetworkData struct {
	CChainTeleporterMessengerAddress string
	CChainTeleporterRegistryAddress  string
}

type ClusterConfig struct {
	Nodes              []string
	APINodes           []string
	Network            Network
	MonitoringInstance string            // instance ID of the separate monitoring instance (if any)
	LoadTestInstance   map[string]string // maps load test name to load test cloud instance ID of the separate load test instance (if any)
	ExtraNetworkData   ExtraNetworkData
	Subnets            []string
	External           bool
	Local              bool
	HTTPAccess         constants.HTTPAccess
}

type ClustersConfig struct {
	Version   string
	KeyPair   map[string]string        // maps key pair name to cert path
	Clusters  map[string]ClusterConfig // maps clusterName to nodeID list + network kind
	GCPConfig GCPConfig                // stores GCP project name and filepath to service account JSON key
}

// GetAPINodes returns a filtered list of API nodes based on the ClusterConfig and given hosts.
func (cc *ClusterConfig) GetAPIHosts(hosts []*Host) []*Host {
	return utils.Filter(hosts, func(h *Host) bool {
		return slices.Contains(cc.APINodes, h.NodeID)
	})
}

// GetValidatorNodes returns the validator nodes from the ClusterConfig.
func (cc *ClusterConfig) GetValidatorHosts(hosts []*Host) []*Host {
	return utils.Filter(hosts, func(h *Host) bool {
		return !slices.Contains(cc.APINodes, h.GetCloudID())
	})
}

func (cc *ClusterConfig) IsAPIHost(hostCloudID string) bool {
	return cc.Local || slices.Contains(cc.APINodes, hostCloudID)
}

func (cc *ClusterConfig) IsAvalancheGoHost(hostCloudID string) bool {
	return cc.Local || slices.Contains(cc.Nodes, hostCloudID)
}

func (cc *ClusterConfig) GetCloudIDs() []string {
	if cc.Local {
		return nil
	}
	r := cc.Nodes
	if cc.MonitoringInstance != "" {
		r = append(r, cc.MonitoringInstance)
	}
	return r
}

func (cc *ClusterConfig) GetHostRoles(nodeConf NodeConfig) []string {
	roles := []string{}
	if cc.IsAvalancheGoHost(nodeConf.NodeID) {
		if cc.IsAPIHost(nodeConf.NodeID) {
			roles = append(roles, constants.APIRole)
		} else {
			roles = append(roles, constants.ValidatorRole)
		}
	}
	if nodeConf.IsMonitor {
		roles = append(roles, constants.MonitorRole)
	}
	if nodeConf.IsAWMRelayer {
		roles = append(roles, constants.AWMRelayerRole)
	}
	return roles
}
