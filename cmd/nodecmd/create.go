// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/cobra"
	"github.com/zclconf/go-cty/cty"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [subnetName]",
		Short: "Create a new subnet configuration",
		Long: `The subnet create command builds a new genesis file to configure your Subnet.
By default, the command runs an interactive wizard. It walks you through
all the steps you need to create your first Subnet.

The tool supports deploying Subnet-EVM, and custom VMs. You
can create a custom, user-generated genesis with a custom VM by providing
the path to your genesis and VM binaries with the --genesis and --vm flags.

By default, running the command with a subnetName that already exists
causes the command to fail. If you’d like to overwrite an existing
configuration, pass the -f flag.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         createNode,
	}

	return cmd
}

func getNewKeyPairName(ec2Svc *ec2.EC2) (string, error) {
	ux.Logger.PrintToUser("What do you want to name your key pair?")
	for {
		newKeyPairName, err := app.Prompt.CaptureString("Key Pair Name")
		if err != nil {
			return "", err
		}
		keyPairExists, err := checkKeyPairExists(ec2Svc, newKeyPairName)
		if err != nil {
			return "", err
		}
		if !keyPairExists {
			return newKeyPairName, nil
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Key Pair named %s already exists", newKeyPairName))
	}
}

func removeExistingTerraformFiles() error {
	nodeConfigFile := "node_config.tf"
	terraformLockFile := ".terraform.lock.hcl"
	terraformStateFile := "terraform.tfstate"
	terraformStateBackupFile := "terraform.tfstate.backup"
	if _, err := os.Stat(nodeConfigFile); err == nil {
		err := os.Remove(nodeConfigFile)
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(terraformLockFile); err == nil {
		err := os.Remove(terraformLockFile)
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(terraformStateFile); err == nil {
		err := os.Remove(terraformStateFile)
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(terraformStateBackupFile); err == nil {
		err := os.Remove(terraformStateBackupFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func getCertFilePath(certName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	certFilePath := homeDir + "/.ssh/" + certName
	return certFilePath, nil
}

func printNoCredentialsOutput() {
	ux.Logger.PrintToUser("No AWS credentials file found in ~/.aws/credentials")
	ux.Logger.PrintToUser("Create a file called 'credentials' with the contents below, and add the file to ~/.aws/ directory")
	ux.Logger.PrintToUser("===========BEGINNING OF FILE===========")
	ux.Logger.PrintToUser("[default]\naws_access_key_id=<AWS_ACCESS_KEY>\naws_secret_access_key=<AWS_SECRET_ACCESS_KEY>")
	ux.Logger.PrintToUser("===========END OF FILE===========")
	ux.Logger.PrintToUser("More info can be found at https://docs.aws.amazon.com/sdkref/latest/guide/file-format.html#file-format-creds")
}

func createNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]

	err := removeExistingTerraformFiles()
	if err != nil {
		return err
	}
	usr, _ := user.Current()
	keyPairName := usr.Username + "-avalanche-cli"
	certName := keyPairName + "-kp.pem"
	securityGroupName := keyPairName + "-sg"

	hclFile := hclwrite.NewEmptyFile()
	tfFile, err := os.Create("node_config.tf")
	if err != nil {
		return err
	}
	rootBody := hclFile.Body()

	creds := credentials.NewSharedCredentials("", "default")
	credValue, err := creds.Get()
	if err != nil {
		printNoCredentialsOutput()
		return err
	}
	err = setCloudCredentials(rootBody, credValue.AccessKeyID, credValue.SecretAccessKey)
	if err != nil {
		return err
	}
	// Load session from shared config
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-2"),
		Credentials: creds,
	},
	)

	// Create new EC2 client
	ec2Svc := ec2.New(sess)
	var useExistingKeyPair bool
	keyPairExists, err := checkKeyPairExists(ec2Svc, keyPairName)
	if err != nil {
		return err
	}
	if !keyPairExists {
		setKeyPair(rootBody, keyPairName, certName)
	} else {
		certFilePath, err := getCertFilePath(certName)
		if err != nil {
			return err
		}
		if checkCertInSshDir(certFilePath) {
			useExistingKeyPair = true
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists", keyPairName))
			keyPairName, err = getNewKeyPairName(ec2Svc)
			if err != nil {
				return err
			}
			certName = keyPairName + "-kp.pem"
			setKeyPair(rootBody, keyPairName, certName)
		}
	}
	securityGroupExists, sg, err := checkSecurityGroupExists(ec2Svc, securityGroupName)
	if err != nil {
		return err
	}
	userIPAddress, err := getIPAddress()
	if err != nil {
		return err
	}
	if !securityGroupExists {
		setSecurityGroup(rootBody, userIPAddress, securityGroupName)
	} else {
		ipInTCP, ipInHTTP := checkCurrentIpInSg(sg, userIPAddress)
		setSecurityGroupRule(rootBody, userIPAddress, *sg.GroupId, ipInTCP, ipInHTTP)
	}
	setElasticIP(rootBody)
	setUpInstance(rootBody, securityGroupName, useExistingKeyPair, keyPairName)
	setOutput(rootBody)
	_, err = tfFile.Write(hclFile.Bytes())
	if err != nil {
		return err
	}
	instanceID, elasticIP, err := runTerraform()
	if err != nil {
		return err
	}
	certFilePath, err := getCertFilePath(certName)
	if err != nil {
		return err
	}

	if !useExistingKeyPair {
		err = handleCerts(certName)
		if err != nil {
			return err
		}
	}

	inventoryPath := "inventories/" + clusterName
	if err := createAnsibleHostInventory(inventoryPath, elasticIP, certFilePath); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)

	if err := runAnsiblePlaybook(inventoryPath); err != nil {
		return err
	}
	PrintResults(instanceID, elasticIP, certFilePath)
	return nil
}

func checkKeyPairExists(ec2Svc *ec2.EC2, kpName string) (bool, error) {
	keyPairInput := &ec2.DescribeKeyPairsInput{
		KeyNames: []*string{
			aws.String(kpName),
		},
	}

	// Call to get detailed information on each instance
	_, err := ec2Svc.DescribeKeyPairs(keyPairInput)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidKeyPair.NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func checkSecurityGroupExists(ec2Svc *ec2.EC2, sgName string) (bool, *ec2.SecurityGroup, error) {
	sgInput := &ec2.DescribeSecurityGroupsInput{
		GroupNames: []*string{
			aws.String(sgName),
		},
	}

	sg, err := ec2Svc.DescribeSecurityGroups(sgInput)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidGroup.NotFound") {
			return false, &ec2.SecurityGroup{}, nil
		}
		return false, &ec2.SecurityGroup{}, err
	}
	return true, sg.SecurityGroups[0], nil
}

func checkCertInSshDir(certFilePath string) bool {
	_, err := os.Stat(certFilePath)
	return err == nil
}

func checkCurrentIpInSg(sg *ec2.SecurityGroup, currentIP string) (bool, bool) {
	var ipInTCP bool
	var ipInHTTP bool
	for _, ip := range sg.IpPermissions {
		if *ip.FromPort == 22 || *ip.FromPort == 9650 {
			for _, cidrIP := range ip.IpRanges {
				if strings.Contains(cidrIP.String(), currentIP) {
					if *ip.FromPort == 22 {
						ipInTCP = true
					} else if *ip.FromPort == 9650 {
						ipInHTTP = true
					}
					break
				}
			}
		}
	}
	return ipInTCP, ipInHTTP
}

func setCloudCredentials(rootBody *hclwrite.Body, accessKey, secretKey string) error {
	provider := rootBody.AppendNewBlock("provider", []string{"aws"})
	providerBody := provider.Body()
	providerBody.SetAttributeValue("access_key", cty.StringVal(accessKey))
	providerBody.SetAttributeValue("secret_key", cty.StringVal(secretKey))
	providerBody.SetAttributeValue("region", cty.StringVal("us-east-2"))

	return nil
}

func setKeyPair(rootBody *hclwrite.Body, keyName, certName string) {
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

func getIPAddress() (string, error) {
	ipOutput, err := exec.Command("curl", "ipecho.net/plain").Output()
	if err != nil {
		return "", err
	}
	ipAddress := string(ipOutput)
	if net.ParseIP(ipAddress) == nil {
		return "", errors.New("invalid IP address")
	}
	return ipAddress, nil
}

func setSecurityGroup(rootBody *hclwrite.Body, ipAddress, securityGroupName string) {
	inputIPAddress := ipAddress + "/32"
	securityGroup := rootBody.AppendNewBlock("resource", []string{"aws_security_group", "ssh_avax_sg"})
	securityGroupBody := securityGroup.Body()
	securityGroupBody.SetAttributeValue("name", cty.StringVal(securityGroupName))
	securityGroupBody.SetAttributeValue("description", cty.StringVal("Allow SSH, AVAX HTTP outbound traffic"))
	inboundGroup := securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody := inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("TCP"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(22))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(22))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	var ipList []cty.Value
	ipList = append(ipList, cty.StringVal(inputIPAddress))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX HTTP"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(9650))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(9650))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal("0.0.0.0/0"))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX HTTP"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(9650))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(9650))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal(inputIPAddress))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("ingress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("AVAX Staking"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(9651))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(9651))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal("0.0.0.0/0"))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))

	inboundGroup = securityGroupBody.AppendNewBlock("egress", []string{})
	inboundGroupBody = inboundGroup.Body()
	inboundGroupBody.SetAttributeValue("description", cty.StringVal("Outbound traffic"))
	inboundGroupBody.SetAttributeValue("from_port", cty.NumberIntVal(0))
	inboundGroupBody.SetAttributeValue("to_port", cty.NumberIntVal(0))
	inboundGroupBody.SetAttributeValue("protocol", cty.StringVal("-1"))
	ipList = []cty.Value{}
	ipList = append(ipList, cty.StringVal("0.0.0.0/0"))
	inboundGroupBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
}

func setSecurityGroupRule(rootBody *hclwrite.Body, ipAddress, sgID string, ipInTcp, ipInHttp bool) {
	inputIPAddress := ipAddress + "/32"
	if !ipInTcp {
		sgRuleName := "ipTcp" + strings.Replace(ipAddress, ".", "", -1)
		securityGroupRule := rootBody.AppendNewBlock("resource", []string{"aws_security_group_rule", sgRuleName})
		securityGroupRuleBody := securityGroupRule.Body()
		securityGroupRuleBody.SetAttributeValue("type", cty.StringVal("ingress"))
		securityGroupRuleBody.SetAttributeValue("from_port", cty.NumberIntVal(22))
		securityGroupRuleBody.SetAttributeValue("to_port", cty.NumberIntVal(22))
		securityGroupRuleBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
		var ipList []cty.Value
		ipList = append(ipList, cty.StringVal(inputIPAddress))
		securityGroupRuleBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
		securityGroupRuleBody.SetAttributeValue("security_group_id", cty.StringVal(sgID))
	}
	if !ipInHttp {
		sgRuleName := "ipHttp" + strings.Replace(ipAddress, ".", "", -1)
		// sgRuleName := "ipHttp"
		securityGroupRule := rootBody.AppendNewBlock("resource", []string{"aws_security_group_rule", sgRuleName})
		securityGroupRuleBody := securityGroupRule.Body()
		securityGroupRuleBody.SetAttributeValue("type", cty.StringVal("ingress"))
		securityGroupRuleBody.SetAttributeValue("from_port", cty.NumberIntVal(9650))
		securityGroupRuleBody.SetAttributeValue("to_port", cty.NumberIntVal(9650))
		securityGroupRuleBody.SetAttributeValue("protocol", cty.StringVal("tcp"))
		var ipList []cty.Value
		ipList = append(ipList, cty.StringVal(inputIPAddress))
		securityGroupRuleBody.SetAttributeValue("cidr_blocks", cty.ListVal(ipList))
		securityGroupRuleBody.SetAttributeValue("security_group_id", cty.StringVal(sgID))
	}
}

func setElasticIP(rootBody *hclwrite.Body) {
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

func setUpInstance(rootBody *hclwrite.Body, securityGroupName string, useExistingKeyPair bool, existingKeyPairName string) {
	awsInstance := rootBody.AppendNewBlock("resource", []string{"aws_instance", "fuji_node"})
	awsInstanceBody := awsInstance.Body()
	awsInstanceBody.SetAttributeValue("count", cty.NumberIntVal(1))
	awsInstanceBody.SetAttributeValue("ami", cty.StringVal("ami-0430580de6244e02e"))
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

func setOutput(rootBody *hclwrite.Body) {
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

func createAnsibleHostInventory(inventoryPath, elasticIP, certFilePath string) error {
	if err := os.MkdirAll(inventoryPath, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	inventoryHostsFile := inventoryPath + "/hosts"
	inventoryFile, err := os.Create(inventoryHostsFile)
	if err != nil {
		return err
	}
	alias := "aws-node "
	elasticIPToUse := elasticIP[1 : len(elasticIP)-2]
	alias += "ansible_host="
	alias += elasticIPToUse
	alias += " ansible_user=ubuntu "
	alias += fmt.Sprintf("ansible_ssh_private_key_file=%s", certFilePath)
	alias += " ansible_ssh_common_args='-o StrictHostKeyChecking=no'"
	_, err = inventoryFile.WriteString(alias + "\n")
	if err != nil {
		return err
	}
	return nil
}

func runTerraform() (string, string, error) {
	var instanceID string
	var elasticIP string
	if err := exec.Command("terraform", "init").Run(); err != nil {
		return "", "", err
	}
	var stdBuffer bytes.Buffer
	cmd := exec.Command("terraform", "apply", "-auto-approve")
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	if err := cmd.Run(); err != nil {
		return "", "", err
	}

	instanceIDOutput, err := exec.Command("terraform", "output", "instance_id").Output()
	if err != nil {
		return "", "", err
	}
	instanceID = string(instanceIDOutput)
	eipOutput, err := exec.Command("terraform", "output", "instance_eip").Output()
	if err != nil {
		return "", "", err
	}
	elasticIP = string(eipOutput)
	return instanceID, elasticIP, nil
}

func handleCerts(certName string) error {
	err := os.Chmod(certName, 0o400)
	if err != nil {
		return err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	certFilePath := homeDir + "/.ssh/" + certName
	err = os.Rename(certName, certFilePath)
	if err != nil {
		return err
	}
	var stdBuffer bytes.Buffer
	cmd := exec.Command("ssh-add", certFilePath)
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func runAnsiblePlaybook(inventoryPath string) error {
	var stdBuffer bytes.Buffer
	cmd := exec.Command("ansible-playbook", "main.yml", "-i", inventoryPath, "--ssh-extra-args='-o IdentitiesOnly=yes'")
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	return cmd.Run()
}

func PrintResults(instanceID, elasticIP, certFilePath string) {
	instanceIDToUse := instanceID[1 : len(instanceID)-2]
	elasticIPToUse := elasticIP[1 : len(elasticIP)-2]
	ux.Logger.PrintToUser("VALIDATOR SUCCESSFULLY SET UP!")
	ux.Logger.PrintToUser("Please wait until validator is successfully boostrapped to run further commands on validator")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Here are the details of the set up validator: ")
	ux.Logger.PrintToUser(fmt.Sprintf("Cloud Instance ID: %s", instanceIDToUse))
	ux.Logger.PrintToUser(fmt.Sprintf("Elastic IP: %s", elasticIPToUse))
	ux.Logger.PrintToUser(fmt.Sprintf("Cloud Region: %s", "us-east-2"))
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("To ssh to validator, run: ")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(fmt.Sprintf("ssh -o IdentitiesOnly=yes ubuntu@%s -i %s", elasticIPToUse, certFilePath))
}
