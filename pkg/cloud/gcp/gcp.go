// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package terraformgcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/compute/v1"

	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type gcpCloud struct {
	gcpClient *compute.Service
	ctx       context.Context
	projectID string
}

func NewGCPCloud(gcpClient *compute.Service, projectID string, ctx context.Context) (*gcpCloud, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return &gcpCloud{
		gcpClient: gcpClient,
		projectID: projectID,
		ctx:       ctx,
	}, nil
}

// waitForOperation waits for a Google Cloud operation to complete.
func (c *gcpCloud) waitForOperation(operation *compute.Operation, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		// Get the status of the operation
		getOperation, err := c.gcpClient.GlobalOperations.Get(c.projectID, operation.Name).Do()
		if err != nil {
			return fmt.Errorf("Error getting operation status: %v", err)
		}

		// Check if the operation has completed
		if getOperation.Status == "DONE" {
			if getOperation.Error != nil {
				return fmt.Errorf("Operation failed: %v", getOperation.Error)
			}
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Operation did not complete within the specified timeout")
		}
		// Wait before checking the status again
		select {
		case <-c.ctx.Done():
			return fmt.Errorf("Operation canceled")
		case <-time.After(1 * time.Second):
		}
	}
}

// SetExistingNetwork uses existing network in GCP
func (c *gcpCloud) SetExistingNetwork(networkName string) (*compute.Network, error) {
	network, err := c.gcpClient.Networks.Get(c.projectID, networkName).Do()
	if err != nil {
		return nil, fmt.Errorf("Error getting network %s: %v", networkName, err)
	}
	return network, nil
}

// SetNetwork creates a new network in GCP
func (c *gcpCloud) SetupNetwork(ipAddress, networkName string) (*compute.Network, error) {
	insertOp, err := c.gcpClient.Networks.Insert(c.projectID, &compute.Network{
		Name: networkName,
	}).Do()
	if err != nil {
		return nil, fmt.Errorf("Error creating network %s: %v", networkName, err)
	}
	if err := c.waitForOperation(insertOp, constants.CloudOperationTimeout); err != nil {
		return nil, err
	}
	// Retrieve the created firewall
	createdNetwork, err := c.gcpClient.Networks.Get(c.projectID, networkName).Do()
	if err != nil {
		return nil, fmt.Errorf("Error retrieving created networks %s: %v", networkName, err)
	}

	// Create firewall rules
	if _, err := c.SetFirewallRule("0.0.0.0/0", fmt.Sprintf("%s-%s", networkName, "default"), networkName, []string{strconv.Itoa(constants.AvalanchegoP2PPort)}, false); err != nil {
		return nil, err
	}
	if _, err := c.SetFirewallRule(ipAddress+"/32", fmt.Sprintf("%s-%s", networkName, strings.ReplaceAll(ipAddress, ".", "")), networkName, []string{strconv.Itoa(constants.SSHTCPPort), strconv.Itoa(constants.AvalanchegoAPIPort)}, false); err != nil {
		return nil, err
	}

	return createdNetwork, nil
}

// SetFirewallRule creates a new firewall rule in GCP
func (c *gcpCloud) SetFirewallRule(ipAddress, firewallName, networkName string, ports []string, networkExists bool) (*compute.Firewall, error) {
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
		return nil, fmt.Errorf("Error creating firewall rule %s: %v", firewallName, err)
	}
	if err := c.waitForOperation(insertOp, constants.CloudOperationTimeout); err != nil {
		return nil, err
	}
	return c.gcpClient.Firewalls.Get(c.projectID, firewallName).Do()
}

// SetPublicIP creates a static IP in GCP
func (c *gcpCloud) SetPublicIP(region, nodeName string, numNodes int) (*compute.Address, error) {
	staticIPName := fmt.Sprintf("%s-%s", "GCPStaticIPPrefix", nodeName)
	address := &compute.Address{
		Name:        staticIPName,
		AddressType: "EXTERNAL",
		NetworkTier: "PREMIUM",
	}

	insertOp, err := c.gcpClient.Addresses.Insert(c.projectID, region, address).Do()
	if err != nil {
		return nil, fmt.Errorf("Error creating static IP %s: %v", staticIPName, err)
	}
	if err := c.waitForOperation(insertOp, constants.CloudOperationTimeout); err != nil {
		return nil, err
	}
	return c.gcpClient.Addresses.Get(c.projectID, region, staticIPName).Do()
}

// SetupInstances creates GCP instances
func (c *gcpCloud) SetupInstances(zone, networkName, sshPublicKey, ami, staticIPName, instanceName string, numNodes int, networkExists bool) (*compute.Instance, error) {
	sshKey := fmt.Sprintf("ubuntu:%s", strings.TrimSuffix(sshPublicKey, "\n"))
	automaticRestart := true
	instance := &compute.Instance{
		Name:        instanceName,
		MachineType: fmt.Sprintf("projects/%s/zones/%s/machineTypes/e2-standard-8", c.projectID, zone),
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
					DiskSizeGb: 1000,
				},
				Boot:       true, // Set this if it's the boot disk
				AutoDelete: true,
				Source:     ami, // Specify the source image here
			},
		},
		Scheduling: &compute.Scheduling{
			AutomaticRestart: &automaticRestart,
		},
	}
	if staticIPName != "" {
		instance.NetworkInterfaces[0].AccessConfigs[0].NatIP = staticIPName
	}

	insertOp, err := c.gcpClient.Instances.Insert(c.projectID, zone, instance).Do()
	if err != nil {
		return nil, fmt.Errorf("Error creating instance %s: %v", instanceName, err)
	}
	if err := c.waitForOperation(insertOp, constants.CloudOperationTimeout); err != nil {
		return nil, err
	}
	return c.gcpClient.Instances.Get(c.projectID, zone, instanceName).Do()
}

// SetExistingNetwork uses existing network in GCP
func SetExistingNetwork(rootBody *hclwrite.Body, networkName string) {
	network := rootBody.AppendNewBlock("data", []string{"google_compute_network", networkName})
	networkBody := network.Body()
	networkBody.SetAttributeValue("name", cty.StringVal(networkName))
}

// SetNetwork houses the firewall (AWS security group equivalent) for GCP
func SetNetwork(rootBody *hclwrite.Body, ipAddress, networkName string) {
	network := rootBody.AppendNewBlock("resource", []string{"google_compute_network", networkName})
	networkBody := network.Body()
	networkBody.SetAttributeValue("name", cty.StringVal(networkName))
	SetFirewallRule(rootBody, "0.0.0.0/0", fmt.Sprintf("%s-%s", networkName, "default"), networkName, []string{strconv.Itoa(constants.AvalanchegoP2PPort)}, false)
	SetFirewallRule(rootBody, ipAddress+"/32", fmt.Sprintf("%s-%s", networkName, strings.ReplaceAll(ipAddress, ".", "")), networkName, []string{strconv.Itoa(constants.SSHTCPPort), strconv.Itoa(constants.AvalanchegoAPIPort)}, false)
}

func SetFirewallRule(rootBody *hclwrite.Body, ipAddress, firewallName, networkName string, ports []string, networkExists bool) {
	firewall := rootBody.AppendNewBlock("resource", []string{"google_compute_firewall", firewallName})
	firewallBody := firewall.Body()
	firewallBody.SetAttributeValue("name", cty.StringVal(firewallName))
	networkRoot := "google_compute_network"
	if networkExists {
		networkRoot = "data.google_compute_network"
	}
	firewallBody.SetAttributeTraversal("network", hcl.Traversal{
		hcl.TraverseRoot{
			Name: networkRoot,
		},
		hcl.TraverseAttr{
			Name: networkName,
		},
		hcl.TraverseAttr{
			Name: "name",
		},
	})
	firewallAllow := firewallBody.AppendNewBlock("allow", []string{})
	firewallAllowBody := firewallAllow.Body()
	firewallAllowBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	allowPortList := []cty.Value{}
	for i := range ports {
		allowPortList = append(allowPortList, cty.StringVal(ports[i]))
	}
	firewallAllowBody.SetAttributeValue("ports", cty.ListVal(allowPortList))
	var allowIPList []cty.Value
	allowIPList = append(allowIPList, cty.StringVal(ipAddress))
	firewallBody.SetAttributeValue("source_ranges", cty.ListVal(allowIPList))
}

// SetPublicIP attach static IP(s) to the associated Google VM instance(s)
func SetPublicIP(rootBody *hclwrite.Body, nodeName string, numNodes int) {
	staticIPName := fmt.Sprintf("%s-%s", constants.GCPStaticIPPrefix, nodeName)
	eip := rootBody.AppendNewBlock("resource", []string{"google_compute_address", staticIPName})
	eipBody := eip.Body()
	eipBody.SetAttributeRaw("name", createCustomTokens(staticIPName))
	eipBody.SetAttributeValue("count", cty.NumberIntVal(int64(numNodes)))
	eipBody.SetAttributeValue("address_type", cty.StringVal("EXTERNAL"))
	eipBody.SetAttributeValue("network_tier", cty.StringVal("PREMIUM"))
}

// createCustomTokens enables usage of ${} in terraform files
func createCustomTokens(tokenName string) hclwrite.Tokens {
	return hclwrite.Tokens{
		{
			Type:  hclsyntax.TokenOQuote,
			Bytes: []byte(`"`),
		},
		{
			Type:  hclsyntax.TokenQuotedLit,
			Bytes: []byte(fmt.Sprintf(`%s-`, tokenName)),
		},
		{
			Type:  hclsyntax.TokenTemplateInterp,
			Bytes: []byte(`${`),
		},
		{
			Type:  hclsyntax.TokenIdent,
			Bytes: []byte(`count`),
		},
		{
			Type:  hclsyntax.TokenDot,
			Bytes: []byte(`.`),
		},
		{
			Type:  hclsyntax.TokenIdent,
			Bytes: []byte(`index`),
		},
		{
			Type:  hclsyntax.TokenTemplateSeqEnd,
			Bytes: []byte(`}`),
		},
		{
			Type:  hclsyntax.TokenCQuote,
			Bytes: []byte(`"`),
		},
	}
}

// SetupInstances adds google_compute_instance section in terraform state file where we configure all the necessary components of the desired GCE instance(s)
func SetupInstances(rootBody *hclwrite.Body, networkName, sshPublicKey, ami, staticIPName, instanceName string, numNodes int, networkExists bool) {
	gcpInstance := rootBody.AppendNewBlock("resource", []string{"google_compute_instance", "gcp-node"})
	gcpInstanceBody := gcpInstance.Body()
	gcpInstanceBody.SetAttributeRaw("name", createCustomTokens(instanceName))
	gcpInstanceBody.SetAttributeValue("count", cty.NumberIntVal(int64(numNodes)))
	gcpInstanceBody.SetAttributeValue("machine_type", cty.StringVal("e2-standard-8"))
	metadataMap := make(map[string]cty.Value)
	metadataMap["ssh-keys"] = cty.StringVal(fmt.Sprintf("ubuntu:%s", strings.TrimSuffix(sshPublicKey, "\n")))
	gcpInstanceBody.SetAttributeValue("metadata", cty.ObjectVal(metadataMap))
	networkInterface := gcpInstanceBody.AppendNewBlock("network_interface", []string{})
	networkInterfaceBody := networkInterface.Body()
	networkRoot := "google_compute_network"
	if networkExists {
		networkRoot = "data.google_compute_network"
	}
	networkInterfaceBody.SetAttributeTraversal("network", hcl.Traversal{
		hcl.TraverseRoot{
			Name: networkRoot,
		},
		hcl.TraverseAttr{
			Name: networkName,
		},
		hcl.TraverseAttr{
			Name: "id",
		},
	})
	accessConfig := networkInterfaceBody.AppendNewBlock("access_config", []string{})
	// don't add google_compute_address if user is not using public IP
	if staticIPName != "" {
		accessConfigBody := accessConfig.Body()
		accessConfigBody.SetAttributeTraversal("nat_ip", hcl.Traversal{
			hcl.TraverseRoot{
				Name: "google_compute_address",
			},
			hcl.TraverseAttr{
				Name: fmt.Sprintf("%s[count.index]", staticIPName),
			},
			hcl.TraverseAttr{
				Name: "address",
			},
		})
	}
	bootDisk := gcpInstanceBody.AppendNewBlock("boot_disk", []string{})
	bootDiskBody := bootDisk.Body()
	initParams := bootDiskBody.AppendNewBlock("initialize_params", []string{})
	initParamsBody := initParams.Body()
	initParamsBody.SetAttributeValue("image", cty.StringVal(ami))
	initParamsBody.SetAttributeValue("size", cty.NumberIntVal(1000))

	gcpInstanceBody.SetAttributeValue("allow_stopping_for_update", cty.BoolVal(true))
}

// SetOutput adds output section in terraform state file so that we can call terraform output command and print instance_ip and instance_id to user
func SetOutput(rootBody *hclwrite.Body) {
	outputEip := rootBody.AppendNewBlock("output", []string{"instance_ips"})
	outputEipBody := outputEip.Body()
	outputEipBody.SetAttributeTraversal("value", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "google_compute_instance",
		},
		hcl.TraverseAttr{
			Name: "gcp-node[*]",
		},
		hcl.TraverseAttr{
			Name: "network_interface.0.access_config.0.nat_ip",
		},
	})
}
