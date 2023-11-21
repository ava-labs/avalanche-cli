// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

type CloudConfig struct {
	InstanceIDs   []string
	PublicIPs     []string
	Region        string
	KeyPair       string
	SecurityGroup string
	CertFilePath  string
	ImageID       string
}

type InstanceConfig struct {
	Prefix            string
	CertName          string
	SecurityGroupName string
	NumNodes          int
	InstanceType      string
}

type CloudConfigMap map[string]CloudConfig

type InstanceConfigMap map[string]InstanceConfig

// GetRegions returns a slice of strings representing the regions of the CloudConfigMap.
func (ccm *CloudConfigMap) GetRegions() []string {
	regions := []string{}
	for _, cloudConfig := range *ccm {
		regions = append(regions, cloudConfig.Region)
	}
	return regions
}

// GetRegions returns a slice of strings containing all the regions associated with the InstanceConfigMap.
func (icm *InstanceConfigMap) GetRegions() []string {
	regions := []string{}
	for region := range *icm {
		regions = append(regions, region)
	}
	return regions
}

// GetInstanceIDs returns a slice of instance IDs based on the specified region.
func (ccm *CloudConfigMap) GetInstanceIDs(region string) []string {
	instanceIDs := []string{}
	for _, cloudConfig := range *ccm {
		if region != "" {
			if cloudConfig.Region == region {
				instanceIDs = append(instanceIDs, cloudConfig.InstanceIDs...)
			}
		} else {
			instanceIDs = append(instanceIDs, cloudConfig.InstanceIDs...)
		}
	}
	return instanceIDs
}
