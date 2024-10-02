// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gcp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"golang.org/x/exp/rand"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/compute/v1"
)

const (
	opScopeZone   = "zone"
	opScopeRegion = "region"
	opScopeGlobal = "global"
	gcpRegionAPI  = "https://www.googleapis.com/compute/v1/projects/%s/regions/%s"
)

var ErrNodeNotFoundToBeRunning = errors.New("node not found to be running")

type GcpCloud struct {
	gcpClient *compute.Service
	ctx       context.Context
	projectID string
}

// NewGcpCloud creates a GCP cloud
func NewGcpCloud(gcpClient *compute.Service, projectID string, ctx context.Context) (*GcpCloud, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return &GcpCloud{
		gcpClient: gcpClient,
		projectID: projectID,
		ctx:       ctx,
	}, nil
}

// getNameFromURL gets the name from the URL
func getNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

// getOperationScope gets the scope of the operation
func getOperationScope(operation *compute.Operation) (string, string) {
	if operation.Zone != "" {
		return opScopeZone, getNameFromURL(operation.Zone)
	} else if operation.Region != "" {
		return opScopeRegion, getNameFromURL(operation.Region)
	}
	return opScopeGlobal, ""
}

// waitForOperation waits for a Google Cloud operation to complete.
func (c *GcpCloud) waitForOperation(operation *compute.Operation) error {
	deadline := time.Now().Add(constants.CloudOperationTimeout)
	for {
		if operation.Status == "DONE" {
			if operation.Error != nil {
				return fmt.Errorf("operation failed: %v", operation.Error)
			}
			return nil
		}
		// Get the status of the operation
		var getOperation *compute.Operation
		var err error
		// Check if the operation is a zone or region specific or global operation
		scope, location := getOperationScope(operation)
		switch {
		case scope == opScopeZone:
			getOperation, err = c.gcpClient.ZoneOperations.Get(c.projectID, location, operation.Name).Do()
		case scope == opScopeRegion:
			getOperation, err = c.gcpClient.RegionOperations.Get(c.projectID, location, operation.Name).Do()
		case scope == opScopeGlobal:
			getOperation, err = c.gcpClient.GlobalOperations.Get(c.projectID, operation.Name).Do()
		default:
			return fmt.Errorf("unknown operation scope: %s", scope)
		}
		if err != nil {
			return fmt.Errorf("error getting operation status: %w", err)
		}
		// Check if the operation has completed
		if getOperation.Status == "DONE" {
			if getOperation.Error != nil {
				return fmt.Errorf("operation failed: %v", getOperation.Error)
			}
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("operation did not complete within the specified timeout")
		}
		// Wait before checking the status again
		select {
		case <-c.ctx.Done():
			return errors.New("operation canceled")
		case <-time.After(1 * time.Second):
			// Continue
		}
	}
}

// SetupNetwork creates a new network in GCP
func (c *GcpCloud) SetupNetwork(ipAddress, networkName string) (*compute.Network, error) {
	insertOp, err := c.gcpClient.Networks.Insert(c.projectID, &compute.Network{
		Name:                  networkName,
		AutoCreateSubnetworks: true, // Use subnet mode
	}).Do()
	if err != nil {
		return nil, fmt.Errorf("error creating network %s: %w", networkName, err)
	}
	if insertOp == nil {
		return nil, fmt.Errorf("error creating network %s: %w", networkName, err)
	} else {
		if err := c.waitForOperation(insertOp); err != nil {
			return nil, err
		}
	}
	// Retrieve the created firewall
	createdNetwork, err := c.gcpClient.Networks.Get(c.projectID, networkName).Do()
	if err != nil {
		return nil, fmt.Errorf("error retrieving created networks %s: %w", networkName, err)
	}

	// Create firewall rules
	if _, err := c.SetFirewallRule("0.0.0.0/0",
		fmt.Sprintf("%s-%s", networkName, "default"),
		networkName,
		[]string{strconv.Itoa(constants.AvalanchegoP2PPort), strconv.Itoa(constants.AvalanchegoLokiPort)}); err != nil {
		return nil, err
	}
	if _, err := c.SetFirewallRule(ipAddress,
		fmt.Sprintf("%s-%s", networkName, strings.ReplaceAll(ipAddress, ".", "")),
		networkName,
		[]string{
			strconv.Itoa(constants.SSHTCPPort), strconv.Itoa(constants.AvalanchegoAPIPort),
			strconv.Itoa(constants.AvalanchegoMonitoringPort), strconv.Itoa(constants.AvalanchegoGrafanaPort),
		}); err != nil {
		return nil, err
	}

	return createdNetwork, nil
}

// SetFirewallRule creates a new firewall rule in GCP
func (c *GcpCloud) SetFirewallRule(ipAddress, firewallName, networkName string, ports []string) (*compute.Firewall, error) {
	if !strings.Contains(ipAddress, "/") {
		ipAddress += "%s/32" // add netmask /32 if missing
	}
	firewall := &compute.Firewall{
		Name:    firewallName,
		Network: fmt.Sprintf("projects/%s/global/networks/%s", c.projectID, networkName),
		Allowed: []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: ports}},
		SourceRanges: []string{
			ipAddress,
		},
	}

	insertOp, err := c.gcpClient.Firewalls.Insert(c.projectID, firewall).Do()
	if err != nil {
		return nil, fmt.Errorf("error creating firewall rule %s: %w", firewallName, err)
	}
	if insertOp == nil {
		return nil, fmt.Errorf("error creating firewall rule %s: %w", firewallName, err)
	} else {
		if err := c.waitForOperation(insertOp); err != nil {
			return nil, err
		}
	}
	return c.gcpClient.Firewalls.Get(c.projectID, firewallName).Do()
}

// SetPublicIP creates a static IP in GCP
func (c *GcpCloud) SetPublicIP(zone, nodeName string, numNodes int) ([]string, error) {
	publicIP := []string{}
	for i := 0; i < numNodes; i++ {
		staticIPName := fmt.Sprintf("%s-%s-%d", constants.GCPStaticIPPrefix, nodeName, i)
		address := &compute.Address{
			Name:        staticIPName,
			AddressType: "EXTERNAL",
			NetworkTier: "PREMIUM",
		}
		region := zoneToRegion(zone)
		insertOp, err := c.gcpClient.Addresses.Insert(c.projectID, region, address).Do()
		if err != nil {
			return nil, fmt.Errorf("error creating static IP 1 %s: %w", staticIPName, err)
		}
		if insertOp == nil {
			return nil, fmt.Errorf("error creating static IP 2 %s", staticIPName)
		} else {
			if err := c.waitForOperation(insertOp); err != nil {
				return nil, err
			}
		}
		computeIP, err := c.gcpClient.Addresses.Get(c.projectID, region, staticIPName).Do()
		if err != nil {
			return nil, fmt.Errorf("error retrieving created static IP %s: %w", staticIPName, err)
		}
		publicIP = append(publicIP, computeIP.Address)
	}

	return publicIP, nil
}

// SetupInstances creates GCP instances
func (c *GcpCloud) SetupInstances(
	cliDefaultName,
	zone,
	networkName,
	sshPublicKey,
	ami,
	instancePrefix,
	instanceType string,
	staticIP []string,
	numNodes int,
	forMonitoring bool,
) ([]*compute.Instance, error) {
	parallelism := 8
	if len(staticIP) > 0 && len(staticIP) != numNodes {
		return nil, errors.New("len(staticIPName) != numNodes")
	}
	instances := make([]*compute.Instance, numNodes)
	instancesChan := make(chan *compute.Instance, numNodes)
	sshKey := "ubuntu:" + strings.TrimSuffix(sshPublicKey, "\n")
	automaticRestart := true

	eg := &errgroup.Group{}
	eg.SetLimit(parallelism)
	for i := 0; i < numNodes; i++ {
		currentIndex := i
		cloudDiskSize := constants.CloudServerStorageSize
		if forMonitoring {
			cloudDiskSize = constants.MonitoringCloudServerStorageSize
		}
		eg.Go(func() error {
			instanceName := fmt.Sprintf("%s-%d", instancePrefix, currentIndex)
			instance := &compute.Instance{
				Name:        instanceName,
				MachineType: fmt.Sprintf("projects/%s/zones/%s/machineTypes/%s", c.projectID, zone, instanceType),
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{Key: "ssh-keys", Value: &sshKey},
					},
				},
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Network: fmt.Sprintf("projects/%s/global/networks/%s", c.projectID, networkName),
						AccessConfigs: []*compute.AccessConfig{
							{
								Name: "External NAT",
							},
						},
					},
				},
				Disks: []*compute.AttachedDisk{
					{
						InitializeParams: &compute.AttachedDiskInitializeParams{
							DiskSizeGb:  int64(cloudDiskSize),
							SourceImage: fmt.Sprintf("projects/%s/global/images/%s", constants.GCPDefaultImageProvider, ami),
						},
						Boot:       true, // Set this if it's the boot disk
						AutoDelete: true,
					},
				},
				Scheduling: &compute.Scheduling{
					AutomaticRestart: &automaticRestart,
				},
				Labels: map[string]string{
					"name":       cliDefaultName,
					"managed-by": "avalanche-cli",
				},
			}
			if staticIP != nil {
				instance.NetworkInterfaces[0].AccessConfigs[0].NatIP = staticIP[currentIndex]
			}
			insertOp, err := c.gcpClient.Instances.Insert(c.projectID, zone, instance).Do()
			if err != nil {
				if isIPLimitExceededError(err) {
					return fmt.Errorf("ip address limit exceeded when creating instance %s: %w", instanceName, err)
				} else {
					return fmt.Errorf("error creating instance %s: %w", instanceName, err)
				}
			}
			if insertOp == nil {
				return fmt.Errorf("error creating instance %s", instanceName)
			} else {
				if err := c.waitForOperation(insertOp); err != nil {
					return fmt.Errorf("error waiting for operation: %w", err)
				}
			}
			inst, err := c.gcpClient.Instances.Get(c.projectID, zone, instanceName).Do()
			if err != nil {
				return fmt.Errorf("error retrieving created instance %s: %w", instanceName, err)
			}
			instancesChan <- inst
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	close(instancesChan)
	for i := 0; i < numNodes; i++ {
		instances[i] = <-instancesChan
	}
	return instances, nil
}

func (c *GcpCloud) GetUbuntuImageID() (string, error) {
	imageListCall := c.gcpClient.Images.List(constants.GCPDefaultImageProvider).Filter(constants.GCPImageFilter)
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
func (c *GcpCloud) CheckFirewallExists(firewallName string, checkMonitoring bool) (bool, error) {
	firewallListCall := c.gcpClient.Firewalls.List(c.projectID)
	firewallList, err := firewallListCall.Do()
	if err != nil {
		return false, err
	}
	for _, firewall := range firewallList.Items {
		if firewall.Name == firewallName {
			if checkMonitoring {
				for _, allowed := range firewall.Allowed {
					if !(slices.Contains(allowed.Ports, strconv.Itoa(constants.AvalanchegoGrafanaPort)) && slices.Contains(allowed.Ports, strconv.Itoa(constants.AvalanchegoMonitoringPort)) && slices.Contains(allowed.Ports, strconv.Itoa(constants.AvalanchegoLokiPort))) {
						return false, nil
					}
				}
			}
			return true, nil
		}
	}
	return false, nil
}

// CheckNetworkExists checks that network networkName exists in GCP project projectName
func (c *GcpCloud) CheckNetworkExists(networkName string) (bool, error) {
	networkListCall := c.gcpClient.Networks.List(c.projectID)
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
func (c *GcpCloud) GetInstancePublicIPs(zone string, nodeIDs []string) (map[string]string, error) {
	instancesListCall := c.gcpClient.Instances.List(c.projectID, zone)
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
func (c *GcpCloud) checkInstanceIsRunning(zone, nodeID string) (bool, error) {
	instanceGetCall := c.gcpClient.Instances.Get(c.projectID, zone, nodeID)
	instance, err := instanceGetCall.Do()
	if err != nil {
		return false, err
	}
	if instance.Status != "RUNNING" {
		return false, fmt.Errorf("error %s is not running", nodeID)
	}
	return true, nil
}

// DestroyGCPNode terminates GCP node in GCP
func (c *GcpCloud) DestroyGCPNode(nodeConfig models.NodeConfig, clusterName string) error {
	isRunning, err := c.checkInstanceIsRunning(nodeConfig.Region, nodeConfig.NodeID)
	if err != nil {
		return err
	}
	if !isRunning {
		return fmt.Errorf("%w: instance %s, cluster %s", ErrNodeNotFoundToBeRunning, nodeConfig.NodeID, clusterName)
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Destroying node instance %s in cluster %s...", nodeConfig.NodeID, clusterName))
	instancesStopCall := c.gcpClient.Instances.Delete(c.projectID, nodeConfig.Region, nodeConfig.NodeID)
	if _, err = instancesStopCall.Do(); err != nil {
		return err
	}
	if nodeConfig.UseStaticIP {
		ux.Logger.PrintToUser(fmt.Sprintf("Releasing static IP address %s ...", nodeConfig.ElasticIP))
		// GCP node region is stored in format of "us-east1-b", we need "us-east1"
		region := strings.Join(strings.Split(nodeConfig.Region, "-")[:2], "-")
		addressReleaseCall := c.gcpClient.Addresses.Delete(c.projectID, region, fmt.Sprintf("%s-%s", constants.GCPStaticIPPrefix, nodeConfig.NodeID))
		if _, err = addressReleaseCall.Do(); err != nil {
			return fmt.Errorf("%s, %w", constants.ErrReleasingGCPStaticIP, err)
		}
	}
	return nil
}

// AddFirewall adds firewall into an existing project in GCP
func (c *GcpCloud) AddFirewall(publicIP, networkName, projectName, firewallName string, ports []string, checkMonitoring bool) error {
	firewallExists, err := c.CheckFirewallExists(firewallName, checkMonitoring)
	if err != nil {
		return err
	}
	if !firewallExists {
		allowedFirewall := compute.FirewallAllowed{
			IPProtocol: "tcp",
			Ports:      ports,
		}
		firewall := compute.Firewall{
			Name:         firewallName,
			Allowed:      []*compute.FirewallAllowed{&allowedFirewall},
			Network:      "global/networks/" + networkName,
			SourceRanges: []string{publicIP + constants.IPAddressSuffix},
		}
		instancesStopCall := c.gcpClient.Firewalls.Insert(projectName, &firewall)
		if _, err = instancesStopCall.Do(); err != nil {
			return err
		}
	}
	return nil
}

// ListRegions returns a list of regions for the GcpCloud instance.
func (c *GcpCloud) ListRegions() []string {
	regionListCall := c.gcpClient.Regions.List(c.projectID)
	regionList, err := regionListCall.Do()
	if err != nil {
		return nil
	}
	regions := []string{}
	for _, region := range regionList.Items {
		regions = append(regions, region.Name)
	}
	return regions
}

// ListZonesInRegion returns a list of zones in a specific region for a given project ID.
func (c *GcpCloud) ListZonesInRegion(region string) ([]string, error) {
	zoneListCall := c.gcpClient.Zones.List(c.projectID)
	zoneList, err := zoneListCall.Do()
	if err != nil {
		return nil, err
	}
	zones := []string{}
	for _, zone := range zoneList.Items {
		if zone.Region == fmt.Sprintf(gcpRegionAPI, c.projectID, region) {
			zones = append(zones, zone.Name)
		}
	}
	return zones, nil
}

// GetRandomZone returns a random zone in the specified region.
func (c *GcpCloud) GetRandomZone(region string) (string, error) {
	rand.Seed(uint64(time.Now().UnixNano()))
	zones, err := c.ListZonesInRegion(region)
	if err != nil {
		return "", fmt.Errorf("error listing zones: %w", err)
	}
	if len(zones) == 0 {
		return "", fmt.Errorf("no zones found in region %s", region)
	}
	return zones[rand.Intn(len(zones))], nil
}

// zoneToRegion returns region from zone
func zoneToRegion(zone string) string {
	splitZone := strings.Split(zone, "-")
	if len(splitZone) < 2 {
		return ""
	}
	return strings.Join(splitZone[:2], "-")
}

// isIPLimitExceededError checks if error is IP limit exceeded
func isIPLimitExceededError(err error) bool {
	return strings.Contains(err.Error(), "IP address quota exceeded") || strings.Contains(err.Error(), "Insufficient IP addresses")
}

// ListAttachedVolumes returns a list of attached volumes to the instance excluding the boot volume
func (c *GcpCloud) GetRootVolumeID(instanceID string, zone string) (string, error) {
	instance, err := c.gcpClient.Instances.Get(c.projectID, zone, instanceID).Do()
	if err != nil {
		return "", err
	}
	for _, disk := range instance.Disks {
		if disk.Boot {
			return extractDiskIDFromURL(disk.Source), nil
		}
	}
	return "", fmt.Errorf("no root volume found for instance %s", instanceID)
}

// extractDiskIDFromURL extracts the disk ID from the disk URL
func extractDiskIDFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

// ResizeVolume resizes the volume to the new size
func (c *GcpCloud) ResizeVolume(volumeID string, zone string, newSizeGb int64) error {
	disk, err := c.gcpClient.Disks.Get(c.projectID, zone, volumeID).Do()
	if err != nil {
		return err
	}
	if disk.SizeGb > newSizeGb {
		return fmt.Errorf("new size %dGb must be greater than the current size %dGb", newSizeGb, disk.SizeGb)
	} else {
		operation, err := c.gcpClient.Disks.Resize(c.projectID, zone, volumeID, &compute.DisksResizeRequest{SizeGb: newSizeGb}).Do()
		if err != nil {
			return err
		}
		if err := c.waitForOperation(operation); err != nil {
			return err
		}
	}
	return nil
}

// ChangeInstanceType changes the instance type of the instance on-the-fly
func (c *GcpCloud) ChangeInstanceType(instanceID, zone, machineType string) error {
	// check if new and current machine types are the same
	instance, err := c.gcpClient.Instances.Get(c.projectID, zone, instanceID).Do()
	if err != nil {
		return err
	}
	currentMachineType := instance.MachineType

	if strings.HasSuffix(currentMachineType, fmt.Sprintf("zones/%s/machineTypes/%s", zone, machineType)) {
		return fmt.Errorf("instance %s is already of type %s", instanceID, machineType)
	}
	// stop the instance
	op, err := c.gcpClient.Instances.Stop(c.projectID, zone, instanceID).Do()
	if err != nil {
		return err
	}
	if err := c.waitForOperation(op); err != nil {
		return err
	}
	// update the machine type
	op, err = c.gcpClient.Instances.SetMachineType(c.projectID, zone, instanceID, &compute.InstancesSetMachineTypeRequest{
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", zone, machineType),
	}).Do()
	if err != nil {
		return err
	}
	if err := c.waitForOperation(op); err != nil {
		return err
	}
	// start the instance
	op, err = c.gcpClient.Instances.Start(c.projectID, zone, instanceID).Do()
	if err != nil {
		return err
	}
	if err := c.waitForOperation(op); err != nil {
		return err
	}

	return nil
}

// IsInstanceTypeSupported checks if the machine type is supported in the zone
func (c *GcpCloud) IsInstanceTypeSupported(machineType string, zone string) (bool, error) {
	machineTypes, err := c.gcpClient.MachineTypes.List(c.projectID, zone).Do()
	if err != nil {
		return false, err
	}
	supportedMachineTypes := utils.Map(machineTypes.Items, func(mt *compute.MachineType) string {
		return mt.Name
	})
	return slices.Contains(supportedMachineTypes, machineType), nil
}
