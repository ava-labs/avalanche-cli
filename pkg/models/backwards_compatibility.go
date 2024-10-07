// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type ClustersConfigV0 struct {
	KeyPair   map[string]string   // maps key pair name to cert path
	Clusters  map[string][]string // maps clusterName to nodeID list
	GCPConfig GCPConfig           // stores GCP project name and filepath to service account JSON key
}
