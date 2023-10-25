// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package terraformgcp

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/terraform"

	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// SetCloudCredentials sets GCP account credentials defined in service account JSON file
func SetCloudCredentials(rootBody *hclwrite.Body, zone, credentialsPath, projectName string) error {
	// zone's format is us-east1-b, region's format is us-east1
	region := strings.Join(strings.Split(zone, "-")[:2], "-")
	provider := rootBody.AppendNewBlock("provider", []string{"google"})
	providerBody := provider.Body()
	providerBody.SetAttributeValue("project", cty.StringVal(projectName))
	providerBody.SetAttributeValue("region", cty.StringVal(region))
	providerBody.SetAttributeValue("zone", cty.StringVal(zone))
	providerBody.SetAttributeValue("credentials", cty.StringVal(credentialsPath))
	return nil
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
	SetFirewallRule(rootBody, "0.0.0.0/0", fmt.Sprintf("%s-%s", networkName, "default"), networkName, []string{strconv.Itoa(constants.AvalanchegoAPIPort), strconv.Itoa(constants.AvalanchegoP2PPort)}, false)
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

// RunTerraform executes terraform apply function that creates the GCE instances based on the .tf file provided
// returns a list of GCP node IPs
func RunTerraform(terraformDir string, useEIP bool) ([]string, error) {
	cmd := exec.Command(constants.Terraform, "init") //nolint:gosec
	cmd.Dir = terraformDir
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	cmd = exec.Command(constants.Terraform, "apply", "-auto-approve") //nolint:gosec
	cmd.Dir = terraformDir
	utils.SetupRealtimeCLIOutput(cmd, true, true)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	var publicIPs []string
	var err error
	if useEIP {
		publicIPs, err = terraform.GetPublicIPs(terraformDir)
		if err != nil {
			return nil, err
		}
	}
	return publicIPs, nil
}
