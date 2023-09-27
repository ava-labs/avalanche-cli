// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type ClusterConfig struct {
	KeyPair           map[string]string   // maps key pair name to cert path
	Clusters          map[string][]string // maps clusterName to nodeID list
	ServiceAccountKey string              // GCP Only: filepath to service account JSON key
}
