// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type NodeConfig struct {
	NodeID        string // instance id on cloud server
	Region        string // region where cloud server instance is deployed
	AMI           string // image id for cloud server dependent on its os (e.g. ubuntu )and region deployed (e.g. us-east-1)
	KeyPair       string // key pair name used on cloud server
	CertPath      string // where the cert is stored in user's local machine ssh directory
	SecurityGroup string // security group used on cloud server
	ElasticIP     string // public IP address of the cloud server
	CloudService  string // which cloud service node is hosted on (AWS / GCP)
}
