// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type ExportNode struct {
	NodeConfig NodeConfig `json:"nodeConfig"`
	SignerKey  string     `json:"signerKey"`
	StakerKey  string     `json:"stakerKey"`
	StakerCrt  string     `json:"stakerCrt"`
}
type ExportCluster struct {
	ClusterConfig ClusterConfig `json:"clusterConfig"`
	Nodes         []ExportNode  `json:"nodes"`
	MonitorNode   ExportNode    `json:"monitorNode"`
	LoadTestNodes []ExportNode  `json:"loadTestNodes"`
}
