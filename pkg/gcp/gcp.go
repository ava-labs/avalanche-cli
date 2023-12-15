// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gcp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
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

func checkFirewallIsForSeparateMonitoring(firewallName string) bool {
	splitFirewall := strings.Split(firewallName, "-")
	if len(splitFirewall) > 0 && splitFirewall[len(splitFirewall)-1] == "monitoring" {
		return true
	}
	return false
}

// CheckFirewallExists checks that firewall firewallName exists in GCP project projectName
func CheckFirewallExists(gcpClient *compute.Service, projectName, firewallName string, checkMonitoring bool) (bool, error) {
	firewallListCall := gcpClient.Firewalls.List(projectName)
	firewallList, err := firewallListCall.Do()
	if err != nil {
		return false, err
	}
	for _, firewall := range firewallList.Items {
		if firewall.Name == firewallName {
			if checkMonitoring {
				for _, allowed := range firewall.Allowed {
					fmt.Printf("allowed")
					if checkFirewallIsForSeparateMonitoring(firewallName) {
						return true, nil
					}
					if !(slices.Contains(allowed.Ports, strconv.Itoa(constants.AvalanchegoGrafanaPort)) && slices.Contains(allowed.Ports, strconv.Itoa(constants.AvalanchegoMonitoringPort))) {
						fmt.Printf("we are returning false here \n")
						return false, nil
					}
				}
			}
			fmt.Printf("we are returning true here \n")
			return true, nil
		}
	}
	fmt.Printf("we are returnign false at the end \n")
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

// checkInstanceIsRunning checks that GCP instance nodeID is running in GCP
func checkInstanceIsRunning(gcpClient *compute.Service, projectName, zone, nodeID string) (bool, error) {
	instanceGetCall := gcpClient.Instances.Get(projectName, zone, nodeID)
	instance, err := instanceGetCall.Do()
	if err != nil {
		return false, err
	}
	if instance.Status != "RUNNING" {
		return false, fmt.Errorf("error %s is not running", nodeID)
	}
	return true, nil
}

func StopGCPNode(gcpClient *compute.Service, nodeConfig models.NodeConfig, projectName, clusterName string, releasePublicIP bool) error {
	isRunning, err := checkInstanceIsRunning(gcpClient, projectName, nodeConfig.Region, nodeConfig.NodeID)
	if err != nil {
		return err
	}
	if !isRunning {
		noRunningNodeErr := fmt.Errorf("no running node with instance id %s is found in cluster %s", nodeConfig.NodeID, clusterName)
		return noRunningNodeErr
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Stopping node instance %s in cluster %s...", nodeConfig.NodeID, clusterName))
	instancesStopCall := gcpClient.Instances.Stop(projectName, nodeConfig.Region, nodeConfig.NodeID)
	if _, err = instancesStopCall.Do(); err != nil {
		return err
	}
	if releasePublicIP && nodeConfig.ElasticIP != "" {
		ux.Logger.PrintToUser(fmt.Sprintf("Releasing static IP address %s ...", nodeConfig.ElasticIP))
		// GCP node region is stored in format of "us-east1-b", we need "us-east1"
		region := strings.Join(strings.Split(nodeConfig.Region, "-")[:2], "-")
		addressReleaseCall := gcpClient.Addresses.Delete(projectName, region, fmt.Sprintf("%s-%s", constants.GCPStaticIPPrefix, nodeConfig.NodeID))
		if _, err = addressReleaseCall.Do(); err != nil {
			return fmt.Errorf("%s, %w", constants.ErrReleasingGCPStaticIP, err)
		}
	}
	return nil
}

func AddFirewall(gcpClient *compute.Service, monitoringHostPublicIP, networkName, projectName string) error {
	firewallName := fmt.Sprintf("%s-%s-monitoring", networkName, strings.ReplaceAll(monitoringHostPublicIP, ".", ""))
	firewallExists, err := CheckFirewallExists(gcpClient, projectName, firewallName, true)
	if err != nil {
		fmt.Printf("we have CheckFirewallExists err %s \n", err)
		return err
	}
	if !firewallExists {
		fmt.Printf("firewall doesn't exists \n")
		allowedFirewall := compute.FirewallAllowed{
			IPProtocol: "tcp",
			Ports:      []string{strconv.Itoa(constants.AvalanchegoMachineMetricsPort), strconv.Itoa(constants.AvalanchegoAPIPort)},
		}

		firewall := compute.Firewall{
			Name:         firewallName,
			Allowed:      []*compute.FirewallAllowed{&allowedFirewall},
			Network:      fmt.Sprintf("global/networks/%s", networkName),
			SourceRanges: []string{monitoringHostPublicIP + constants.IPAddressSuffix},
		}
		instancesStopCall := gcpClient.Firewalls.Insert(projectName, &firewall)
		if _, err = instancesStopCall.Do(); err != nil {
			return err
		}
	} else {
		fmt.Printf("firewall exists \n")
	}
	return nil
}
