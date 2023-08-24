// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package terraform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// InitConf creates hclFile where we define all terraform configuration in hclFile.Body() and create .tf file where we save the content in
func InitConf() (*hclwrite.File, *hclwrite.Body, error) {
	hclFile := hclwrite.NewEmptyFile()
	rootBody := hclFile.Body()
	return hclFile, rootBody, nil
}

// SaveConf writes all terraform configuration defined in hclFile to tfFile
func SaveConf(terraformDir string, hclFile *hclwrite.File) error {
	tfFile, err := os.Create(filepath.Join(terraformDir, constants.TerraformNodeConfigFile))
	if err != nil {
		return err
	}
	_, err = tfFile.Write(hclFile.Bytes())
	return err
}

// SetCloudCredentials sets AWS account credentials defined in .aws dir in user home dir
func SetCloudCredentials(rootBody *hclwrite.Body, region string) error {
	provider := rootBody.AppendNewBlock("provider", []string{"aws"})
	providerBody := provider.Body()
	providerBody.SetAttributeValue("region", cty.StringVal(region))
	providerBody.SetAttributeValue("profile", cty.StringVal("default"))
	return nil
}

// addSecurityGroupRuleToSg is to add sg rule to new sg
func addSecurityGroupRuleToSg(securityGroupBody *hclwrite.Body, sgType, description, protocol, ip string, port int64) {
	inboundGroup := securityGroupBody.AppendNewBlock(sgType, []string{})
	inboundGroupBody := inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal(description))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(port))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(port))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal(protocol))
	var ipList []cty.Value
	ipList = append(ipList, cty.StringVal(ip))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
}

// addNewSecurityGroupRule is to add sg rule to existing sg
func addNewSecurityGroupRule(rootBody *hclwrite.Body, sgRuleName, sgID, sgType, protocol, ip string, port int64) {
	securityGroupRule := rootBody.AppendNewBlock("resource", []string{"aws_security_group_rule", sgRuleName})
	securityGroupRuleBody := securityGroupRule.Body()
	securityGroupRuleBody.SetAttributeValue("type", cty.StringVal(sgType))
	securityGroupRuleBody.SetAttributeValue("from_port", cty.NumberIntVal(port))
	securityGroupRuleBody.SetAttributeValue("to_port", cty.NumberIntVal(port))
	securityGroupRuleBody.SetAttributeValue("protocol", cty.StringVal(protocol))
	var ipList []cty.Value
	ipList = append(ipList, cty.StringVal(ip))
	securityGroupRuleBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
	securityGroupRuleBody.SetAttributeValue("security_group_id", cty.StringVal(sgID))
}

// SetSecurityGroup whitelists the ip addresses allowed to ssh into cloud server
func SetSecurityGroup(rootBody *hclwrite.Body, ipAddress, securityGroupName string) {
	inputIPAddress := ipAddress + "/32"
	securityGroup := rootBody.AppendNewBlock("resource", []string{"aws_security_group", "ssh_avax_sg"})
	securityGroupBody := securityGroup.Body()
	securityGroupBody.SetAttributeValue("name", cty.StringVal(securityGroupName))
	securityGroupBody.SetAttributeValue("description", cty.StringVal("Allow SSH, AVAX HTTP outbound traffic"))

	// enable inbound access for ip address inputIPAddress in port 22
	addSecurityGroupRuleToSg(securityGroupBody, "ingress", "TCP", "tcp", inputIPAddress, constants.SSHTCPPort)
	// "0.0.0.0/0" is a must-have ip address value for inbound and outbound calls
	addSecurityGroupRuleToSg(securityGroupBody, "ingress", "AVAX HTTP", "tcp", "0.0.0.0/0", constants.AvalanchegoAPIPort)
	// enable inbound access for ip address inputIPAddress in port 9650
	addSecurityGroupRuleToSg(securityGroupBody, "ingress", "AVAX HTTP", "tcp", inputIPAddress, constants.AvalanchegoAPIPort)
	// "0.0.0.0/0" is a must-have ip address value for inbound and outbound calls
	addSecurityGroupRuleToSg(securityGroupBody, "ingress", "AVAX Staking", "tcp", "0.0.0.0/0", constants.AvalanchegoP2PPort)
	addSecurityGroupRuleToSg(securityGroupBody, "egress", "Outbound traffic", "-1", "0.0.0.0/0", constants.OutboundPort)
}

func SetSecurityGroupRule(rootBody *hclwrite.Body, ipAddress, sgID string, ipInTCP, ipInHTTP bool) {
	inputIPAddress := ipAddress + "/32"
	if !ipInTCP {
		sgRuleName := "ipTcp" + strings.ReplaceAll(ipAddress, ".", "")
		addNewSecurityGroupRule(rootBody, sgRuleName, sgID, "ingress", "tcp", inputIPAddress, constants.SSHTCPPort)
	}
	if !ipInHTTP {
		sgRuleName := "ipHttp" + strings.ReplaceAll(ipAddress, ".", "")
		addNewSecurityGroupRule(rootBody, sgRuleName, sgID, "ingress", "tcp", inputIPAddress, constants.AvalanchegoAPIPort)
	}
}

// SetElasticIP attach elastic IP to our ec2 instance
func SetElasticIP(rootBody *hclwrite.Body) {
	eip := rootBody.AppendNewBlock("resource", []string{"aws_eip", "myeip"})
	eipBody := eip.Body()
	eipBody.SetAttributeValue("vpc", cty.BoolVal(true))

	eipAssoc := rootBody.AppendNewBlock("resource", []string{"aws_eip_association", "eip_assoc"})
	eipAssocBody := eipAssoc.Body()
	eipAssocBody.SetAttributeTraversal("instance_id", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_instance",
		},
		hcl.TraverseAttr{
			Name: "aws_node[0]",
		},
		hcl.TraverseAttr{
			Name: "id",
		},
	})
	eipAssocBody.SetAttributeTraversal("allocation_id", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_eip",
		},
		hcl.TraverseAttr{
			Name: "myeip",
		},
		hcl.TraverseAttr{
			Name: "id",
		},
	})
}

// SetKeyPair define the key pair that we will create in our EC2 instance if it doesn't exist yet and download the .pem file to home dir
func SetKeyPair(rootBody *hclwrite.Body, keyName, certName string) {
	// define the encryption we are using for the key pair
	tlsPrivateKey := rootBody.AppendNewBlock("resource", []string{"tls_private_key", "pk"})
	tlsPrivateKeyBody := tlsPrivateKey.Body()
	tlsPrivateKeyBody.SetAttributeValue("algorithm", cty.StringVal("RSA"))
	tlsPrivateKeyBody.SetAttributeValue("rsa_bits", cty.NumberIntVal(4096))

	// define the encryption we are using for the key pair
	keyPair := rootBody.AppendNewBlock("resource", []string{"aws_key_pair", "kp"})
	keyPairBody := keyPair.Body()
	keyPairBody.SetAttributeValue("key_name", cty.StringVal(keyName))
	keyPairBody.SetAttributeTraversal("public_key", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "tls_private_key",
		},
		hcl.TraverseAttr{
			Name: "pk",
		},
		hcl.TraverseAttr{
			Name: "public_key_openssh",
		},
	})

	tfKey := rootBody.AppendNewBlock("resource", []string{"local_file", "tf-key"})
	tfKeyBody := tfKey.Body()
	tfKeyBody.SetAttributeValue("filename", cty.StringVal(certName))
	tfKeyBody.SetAttributeTraversal("content", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "tls_private_key",
		},
		hcl.TraverseAttr{
			Name: "pk",
		},
		hcl.TraverseAttr{
			Name: "private_key_pem",
		},
	})
}

// SetupInstance adds aws_instance section in terraform state file where we configure all the necessary components of the desired ec2 instance
func SetupInstance(rootBody *hclwrite.Body, securityGroupName string, useExistingKeyPair bool, existingKeyPairName, ami string) {
	awsInstance := rootBody.AppendNewBlock("resource", []string{"aws_instance", "aws_node"})
	awsInstanceBody := awsInstance.Body()
	awsInstanceBody.SetAttributeValue("count", cty.NumberIntVal(1))
	awsInstanceBody.SetAttributeValue("ami", cty.StringVal(ami))
	awsInstanceBody.SetAttributeValue("instance_type", cty.StringVal("c5.2xlarge"))
	if !useExistingKeyPair {
		awsInstanceBody.SetAttributeTraversal("key_name", hcl.Traversal{
			hcl.TraverseRoot{
				Name: "aws_key_pair",
			},
			hcl.TraverseAttr{
				Name: "kp",
			},
			hcl.TraverseAttr{
				Name: "key_name",
			},
		})
	} else {
		awsInstanceBody.SetAttributeValue("key_name", cty.StringVal(existingKeyPairName))
	}
	var securityGroupList []cty.Value
	securityGroupList = append(securityGroupList, cty.StringVal(securityGroupName))
	awsInstanceBody.SetAttributeValue("security_groups", cty.ListVal(securityGroupList))
	rootBlockDevice := awsInstanceBody.AppendNewBlock("root_block_device", []string{})
	rootBlockDeviceBody := rootBlockDevice.Body()
	rootBlockDeviceBody.SetAttributeValue("volume_size", cty.NumberIntVal(constants.CloudServerStorageSize))
}

// SetOutput adds output section in terraform state file so that we can call terraform output command and print instance_ip and instance_id to user
func SetOutput(rootBody *hclwrite.Body) {
	outputEip := rootBody.AppendNewBlock("output", []string{"instance_ip"})
	outputEipBody := outputEip.Body()
	outputEipBody.SetAttributeTraversal("value", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_eip",
		},
		hcl.TraverseAttr{
			Name: "myeip",
		},
		hcl.TraverseAttr{
			Name: "public_ip",
		},
	})

	outputInstanceID := rootBody.AppendNewBlock("output", []string{"instance_id"})
	outputInstanceIDBody := outputInstanceID.Body()
	outputInstanceIDBody.SetAttributeTraversal("value", hcl.Traversal{
		hcl.TraverseRoot{
			Name: "aws_instance",
		},
		hcl.TraverseAttr{
			Name: "aws_node[0]",
		},
		hcl.TraverseAttr{
			Name: "id",
		},
	})
}

// RemoveDirectory remove terraform directory in .avalanche-cli. We need to call this before and after creating ec2 instance
func RemoveDirectory(terraformDir string) error {
	return os.RemoveAll(terraformDir)
}

// RunTerraform executes terraform apply function that creates the EC2 instances based on the .tf file provided
// returns the AWS node-ID and node IP
func RunTerraform(terraformDir string) (string, string, error) {
	cmd := exec.Command(constants.Terraform, "init") //nolint:gosec
	cmd.Dir = terraformDir
	if err := cmd.Run(); err != nil {
		return "", "", err
	}
	cmd = exec.Command(constants.Terraform, "apply", "-auto-approve") //nolint:gosec
	cmd.Dir = terraformDir
	utils.SetupRealtimeCLIOutput(cmd)
	if err := cmd.Run(); err != nil {
		cmdOutput, cmdOutputErr := cmd.Output()
		if cmdOutputErr != nil {
			fmt.Printf("cmdoutput err %s \n", cmdOutputErr)
			return "", "", cmdOutputErr
		}
		if strings.Contains(string(cmdOutput), "AddressLimitExceeded") {
			fmt.Printf("AddressLimitExceeded found \n")
		}

		return "", "", err
	}
	instanceID, err := GetInstanceID(terraformDir)
	if err != nil {
		return "", "", err
	}
	publicIP, err := GetPublicIP(terraformDir)
	if err != nil {
		return "", "", err
	}
	// eip and nodeID both are bounded by double quotation "", we need to remove them before they can be used
	return instanceID, publicIP, nil
}

func GetInstanceID(terraformDir string) (string, error) {
	cmd := exec.Command(constants.Terraform, "output", "instance_id") //nolint:gosec
	cmd.Dir = terraformDir
	instanceIDOutput, err := cmd.Output()
	if err != nil {
		return "", err
	}
	instanceID := string(instanceIDOutput)
	// eip and nodeID both are bounded by double quotation "", we need to remove them before they can be used
	return instanceID[1 : len(instanceID)-2], nil
}

func GetPublicIP(terraformDir string) (string, error) {
	cmd := exec.Command(constants.Terraform, "output", "instance_ip") //nolint:gosec
	cmd.Dir = terraformDir
	eipOutput, err := cmd.Output()
	if err != nil {
		return "", err
	}
	publicIP := string(eipOutput)
	// eip and nodeID both are bounded by double quotation "", we need to remove them before they can be used
	return publicIP[1 : len(publicIP)-2], nil
}

func CheckIsInstalled() error {
	if err := exec.Command(constants.Terraform).Run(); errors.Is(err, exec.ErrNotFound) { //nolint:gosec
		ux.Logger.PrintToUser("Terraform tool is not available. It is a needed dependency for CLI to create a remote node.")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("Please follow install instructions at https://developer.hashicorp.com/terraform/downloads?product_intent=terraform and try again")
		ux.Logger.PrintToUser("")
		return err
	}
	return nil
}
