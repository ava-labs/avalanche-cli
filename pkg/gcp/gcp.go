// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gcp

import (
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"golang.org/x/exp/slices"
	"google.golang.org/api/compute/v1"
)

func GetUbuntuImageID(gcpClient *compute.Service) (string, error) {
	imageListCall := gcpClient.Images.List(constants.GCPDefaultImageProvider).Filter(constants.GCPImageFilter)
	imageList, err := imageListCall.Do()
	if err != nil {
		return "", err
	}
	imageID := ""
	for _, image := range imageList.Items {
		if image.Deprecated == nil {
			imageID = image.Name
			break
		}
	}
	return imageID, nil
}

// CheckFirewallExists checks that firewall firewallName exists in GCP project projectName
func CheckFirewallExists(gcpClient *compute.Service, projectName, firewallName string) (bool, error) {
	firewallListCall := gcpClient.Firewalls.List(projectName)
	firewallList, err := firewallListCall.Do()
	if err != nil {
		return false, err
	}
	for _, firewall := range firewallList.Items {
		if firewall.Name == firewallName {
			return true, nil
		}
	}
	return false, nil
}

// CheckNetworkExists checks that network networkName exists in GCP project projectName
func CheckNetworkExists(gcpClient *compute.Service, projectName, networkName string) (bool, error) {
	networkListCall := gcpClient.Networks.List(projectName)
	networkList, err := networkListCall.Do()
	if err != nil {
		return false, err
	}
	for _, network := range networkList.Items {
		if network.Name == networkName {
			return true, nil
		}
	}
	return false, nil
}

// GetInstancePublicIPs gets public IP(s) of GCP instance(s) without static IP and returns a map
// with gcp instance id as key and public ip as value
func GetInstancePublicIPs(gcpClient *compute.Service, projectName, zone string, nodeIDs []string) (map[string]string, error) {
	instancesListCall := gcpClient.Instances.List(projectName, zone)
	instancesList, err := instancesListCall.Do()
	if err != nil {
		return nil, err
	}
	instanceIDToIP := make(map[string]string)
	for _, instance := range instancesList.Items {
		if slices.Contains(nodeIDs, instance.Name) {
			if len(instance.NetworkInterfaces) > 0 && len(instance.NetworkInterfaces[0].AccessConfigs) > 0 {
				instanceIDToIP[instance.Name] = instance.NetworkInterfaces[0].AccessConfigs[0].NatIP
			}
		}
	}
	return instanceIDToIP, nil
}
