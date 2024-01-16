// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import "golang.org/x/exp/maps"

type RegionConfig struct {
	InstanceIDs       []string
	PublicIPs         []string
	KeyPair           string
	SecurityGroup     string
	CertFilePath      string
	ImageID           string
	Prefix            string
	CertName          string
	SecurityGroupName string
	NumNodes          int
	InstanceType      string
}

type CloudConfig map[string]RegionConfig

// GetRegions returns a slice of strings representing the regions of the RegionConfig.
func (ccm *CloudConfig) GetRegions() []string {
	return maps.Keys(*ccm)
}

// GetAllInstanceIDs returns all instance IDs
func (ccm *CloudConfig) GetAllInstanceIDs() []string {
	instanceIDs := []string{}
	for _, cloudConfig := range *ccm {
		instanceIDs = append(instanceIDs, cloudConfig.InstanceIDs...)
	}
	return instanceIDs
}

// GetInstanceIDsForRegion returns instance IDs for specific region
func (ccm *CloudConfig) GetInstanceIDsForRegion(region string) []string {
	if regionConf, ok := (*ccm)[region]; ok {
		return regionConf.InstanceIDs
	}
	return []string{}
}
