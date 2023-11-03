// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type GCPConfig struct {
	ProjectName        string // name of GCP Project
	ServiceAccFilePath string // location of GCP service account key file path
}

type ClusterConfig struct {
	KeyPair   map[string]string   // maps key pair name to cert path
	Clusters  map[string][]string // maps clusterName to nodeID list
	GCPConfig GCPConfig           // stores GCP project name and filepath to service account JSON key
}
