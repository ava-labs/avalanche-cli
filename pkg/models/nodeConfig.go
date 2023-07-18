// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type NodeConfig struct {
	NodeID        string
	Region        string
	AMI           string
	KeyPair       string
	CertPath      string
	SecurityGroup string
	ElasticIP     string
}
