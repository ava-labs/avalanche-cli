// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package terraform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/utils"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

func CreateTerraformFile(terraformDir string) (*hclwrite.File, *os.File, *hclwrite.Body, error) {
	hclFile := hclwrite.NewEmptyFile()
	terraformFilePath := terraformDir + "/node_config.tf"
	tfFile, err := os.Create(terraformFilePath)
	if err != nil {
		return nil, nil, nil, err
	}
	rootBody := hclFile.Body()
	return hclFile, tfFile, rootBody, nil
}

func SaveTerraformFile(tfFile *os.File, hclFile *hclwrite.File) error {
	_, err := tfFile.Write(hclFile.Bytes())
	if err != nil {
		return err
	}
	return nil
}

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
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.TCPPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.TCPPort))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	var ipList []cty.Value
	ipList = append(ipList, cty.StringVal(inputIPAddress))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX HTTP"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.HTTPPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.HTTPPort))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal("0.0.0.0/0"))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX HTTP"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.HTTPPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.HTTPPort))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal(inputIPAddress))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX Staking"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.StakingPort))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.StakingPort))
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
		securityGroupRuleBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.TCPPort))
		securityGroupRuleBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.TCPPort))
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
		securityGroupRuleBody.SetAttributeValue("from_port", cty.NumberIntVal(constants.HTTPPort))
		securityGroupRuleBody.SetAttributeValue("to_port", cty.NumberIntVal(constants.HTTPPort))
		securityGroupRuleBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
		var ipList []cty.Value
		ipList = append(ipList, cty.StringVal(inputIPAddress))
		securityGroupRuleBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
		securityGroupRuleBody.SetAttributeValue("security_group_id", cty.StringVal(sgID))
	}
}

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
			Name: "fuji_node[0]",
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

func SetKeyPair(rootBody *hclwrite.Body, keyName, certName string) {
	tlsPrivateKey := rootBody.AppendNewBlock("resource", []string{"tls_private_key", "pk"})
	tlsPrivateKeyBody := tlsPrivateKey.Body()
	tlsPrivateKeyBody.SetAttributeValue("algorithm", cty.StringVal("RSA"))
	tlsPrivateKeyBody.SetAttributeValue("rsa_bits", cty.NumberIntVal(4096))

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

func SetUpInstance(rootBody *hclwrite.Body, securityGroupName string, useExistingKeyPair bool, existingKeyPairName, ami string) {
	awsInstance := rootBody.AppendNewBlock("resource", []string{"aws_instance", "fuji_node"})
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

func SetOutput(rootBody *hclwrite.Body) {
	outputEip := rootBody.AppendNewBlock("output", []string{"instance_eip"})
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
			Name: "fuji_node[0]",
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

func RemoveExistingTerraformFiles(terraformDir string) error {
	err := removeFile(terraformDir + constants.NodeConfigFile)
	if err != nil {
		return err
	}
	err = removeFile(terraformDir + constants.TerraformLockFile)
	if err != nil {
		return err
	}
	err = removeFile(terraformDir + constants.TerraformStateFile)
	if err != nil {
		return err
	}
	return removeFile(terraformDir + constants.TerraformStateBackupFile)
}

func RunTerraform(terraformDir string) (string, string, error) {
	var instanceID string
	var elasticIP string
	cmd := exec.Command(constants.Terraform, "init") //nolint:gosec
	cmd.Dir = terraformDir
	if err := cmd.Run(); err != nil {
		fmt.Printf("error here backendConfigVar %s \n", err)
		return "", "", err
	}
	cmd = exec.Command("terraform", "apply", "-auto-approve")
	cmd.Dir = terraformDir
	utils.SetUpMultiWrite(cmd)
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
	cmd = exec.Command(constants.Terraform, "output", "instance_eip") //nolint:gosec
	cmd.Dir = terraformDir
	eipOutput, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	elasticIP = string(eipOutput)
	return instanceID, elasticIP, nil
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
