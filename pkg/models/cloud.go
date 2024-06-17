// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import (
	sdkHost "github.com/ava-labs/avalanche-tooling-sdk-go/host"
	"golang.org/x/exp/maps"
)

type RegionConfig struct {
	InstanceIDs       []string
	APIInstanceIDs    []string
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

func (rc *RegionConfig) GetHostCloudParams(cloudService sdkHost.SupportedCloud, region string) sdkHost.CloudParams {
	switch cloudService {
	case sdkHost.AWSCloud:
		return sdkHost.CloudParams{
			Region: region,
			Image:  rc.ImageID,
		}
	default:
		return sdkHost.CloudParams{}
	}
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

// GetAPIInstanceIDsForRegion returns API instance IDs for specific region
func (ccm *CloudConfig) GetAPIInstanceIDsForRegion(region string) []string {
	if regionConf, ok := (*ccm)[region]; ok {
		return regionConf.APIInstanceIDs
	}
	return []string{}
}
