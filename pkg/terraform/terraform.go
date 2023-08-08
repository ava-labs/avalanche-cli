// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package terraform

import (
	"errors"
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

func SetSecurityGroup(rootBody *hclwrite.Body, ipAddress, securityGroupName string) {
	inputIPAddress := ipAddress + "/32"
	securityGroup := rootBody.AppendNewBlock("resource", []string{"aws_security_group", "ssh_avax_sg"})
	securityGroupBody := securityGroup.Body()
	securityGroupBody.SetAttributeValue("name", cty.StringVal(securityGroupName))
	securityGroupBody.SetAttributeValue("description", cty.StringVal("Allow SSH, AVAX HTTP outbound traffic"))

	inboundGroup := securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody := inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("TCP"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.SSHTCPPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.SSHTCPPort))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	var ipList []cty.Value
	ipList = append(ipList, cty.StringVal(inputIPAddress))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX HTTP"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.AvalanchegoAPIPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.AvalanchegoAPIPort))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal("0.0.0.0/0"))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX HTTP"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.AvalanchegoAPIPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.AvalanchegoAPIPort))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal(inputIPAddress))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX Staking"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.AvalanchegoP2PPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.AvalanchegoP2PPort))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal("0.0.0.0/0"))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("egress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("Outbound traffic"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.OutboundPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.OutboundPort))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("-1"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal("0.0.0.0/0"))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
}

func SetSecurityGroupRule(rootBody *hclwrite.Body, ipAddress, sgID string, ipInTCP, ipInHTTP bool) {
	inputIPAddress := ipAddress + "/32"
	if !ipInTCP {
		sgRuleName := "ipTcp" + strings.ReplaceAll(ipAddress, ".", "")
		securityGroupRule := rootBody.AppendNewBlock("resource", []string{"aws_security_group_rule", sgRuleName})
		securityGroupRuleBody := securityGroupRule.Body()
		securityGroupRuleBody.SetAttributeValue("type", cty.StringVal("ingress"))
		securityGroupRuleBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.SSHTCPPort))
		securityGroupRuleBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.SSHTCPPort))
		securityGroupRuleBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
		var ipList []cty.Value
		ipList = append(ipList, cty.StringVal(inputIPAddress))
		securityGroupRuleBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
		securityGroupRuleBody.SetAttributeValue("security_group_id", cty.StringVal(sgID))
	}
	if !ipInHTTP {
		sgRuleName := "ipHttp" + strings.ReplaceAll(ipAddress, ".", "")
		securityGroupRule := rootBody.AppendNewBlock("resource", []string{"aws_security_group_rule", sgRuleName})
		securityGroupRuleBody := securityGroupRule.Body()
		securityGroupRuleBody.SetAttributeValue("type", cty.StringVal("ingress"))
		securityGroupRuleBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.AvalanchegoAPIPort))
		securityGroupRuleBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.AvalanchegoAPIPort))
		securityGroupRuleBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
		var ipList []cty.Value
		ipList = append(ipList, cty.StringVal(inputIPAddress))
		securityGroupRuleBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
		securityGroupRuleBody.SetAttributeValue("security_group_id", cty.StringVal(sgID))
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

func removeFile(fileName string) error {
	if _, err := os.Stat(fileName); err == nil {
		return os.Remove(fileName)
	}
	return nil
}

// RemoveDirectory remove terraform directory in .avalanche-cli. We need to call this before and after creating ec2 instance
func RemoveDirectory(terraformDir string) error {
	return os.RemoveAll(terraformDir)
}

// RunTerraform executes terraform apply function that creates the EC2 instances based on the .tf file provided
// returns the AWS node-ID and node IP
func RunTerraform(terraformDir string) (string, string, error) {
	var instanceID string
	var publicIP string
	cmd := exec.Command(constants.Terraform, "init") //nolint:gosec
	cmd.Dir = terraformDir
	if err := cmd.Run(); err != nil {
		return "", "", err
	}
	cmd = exec.Command(constants.Terraform, "apply", "-auto-approve") //nolint:gosec
	cmd.Dir = terraformDir
	utils.SetupRealtimeCLIOutput(cmd)
	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	cmd = exec.Command(constants.Terraform, "output", "instance_id") //nolint:gosec
	cmd.Dir = terraformDir
	instanceIDOutput, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	instanceID = string(instanceIDOutput)
	cmd = exec.Command(constants.Terraform, "output", "instance_ip") //nolint:gosec
	cmd.Dir = terraformDir
	eipOutput, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	publicIP = string(eipOutput)
	// eip and nodeID both are bounded by double quotation "", we need to remove them before they can be used
	return instanceID[1 : len(instanceID)-2], publicIP[1 : len(publicIP)-2], nil
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
