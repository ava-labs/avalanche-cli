// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
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

func createNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]

	usr, _ := user.Current()
	keyPairName := usr.Username + "-avalanche-cli10"
	certName := keyPairName + "-kp.pem"
	securityGroupName := keyPairName + "-sg"

	hclFile := hclwrite.NewEmptyFile()
	tfFile, err := os.Create("node_config.tf")
	if err != nil {
		return err
	}
	rootBody := hclFile.Body()

	err = setCloudCredentials(rootBody)
	if err != nil {
		return err
	}
	setKeyPair(rootBody, keyPairName, certName)
	err = setSecurityGroup(rootBody, securityGroupName)
	if err != nil {
		return err
	}
	setElasticIP(rootBody)
	setUpInstance(rootBody, securityGroupName)
	setOutput(rootBody)
	_, err = tfFile.Write(hclFile.Bytes())
	if err != nil {
		return err
	}
	instanceID, elasticIP, err := runTerraform()
	if err != nil {
		return err
	}
	certFilePath, err := handleCerts(certName)
	if err != nil {
		return err
	}
	inventoryPath := "inventories/" + clusterName
	if err := createAnsibleHostInventory(inventoryPath, elasticIP, certFilePath); err != nil {
		return err
	}
	if err := runAnsiblePlaybook(inventoryPath); err != nil {
		return err
	}
	PrintResults(instanceID, elasticIP, certFilePath)
	return nil
}

func checkKeyPairExists() {
	// Load session from shared config
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create new EC2 client
	ec2Svc := ec2.New(sess)

	// Call to get detailed information on each instance
	result, err := ec2Svc.DescribeInstances(nil)
	if err != nil {
		fmt.Println("Error", err)
	} else {
		fmt.Println("Success", result)
	}
}

func getCloudCredentials(promptItem, promptStr string) (string, error) {
	ux.Logger.PrintToUser(promptStr)
	tokenName, err := app.Prompt.CaptureString(promptItem)
	if err != nil {
		return "", err
	}
	return tokenName, nil
}

func setCloudCredentials(rootBody *hclwrite.Body) error {
	accessKey, err := getCloudCredentials("AWS Access Key", "Enter your AWS Access Key")
	if err != nil {
		return err
	}
	secretKey, err := getCloudCredentials("Secret Key", "Enter your Secret Key")
	if err != nil {
		return err
	}
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

func setSecurityGroup(rootBody *hclwrite.Body, securityGroupName string) error {
	userIPAddress, err := getIPAddress()
	if err != nil {
		return err
	}
	inputIPAddress := userIPAddress + "/32"
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
	return nil
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

func setUpInstance(rootBody *hclwrite.Body, securityGroupName string) {
	awsInstance := rootBody.AppendNewBlock("resource", []string{"aws_instance", "fuji_node"})
	awsInstanceBody := awsInstance.Body()
	awsInstanceBody.SetAttributeValue("count", cty.NumberIntVal(1))
	awsInstanceBody.SetAttributeValue("ami", cty.StringVal("ami-0430580de6244e02e"))
	awsInstanceBody.SetAttributeValue("instance_type", cty.StringVal("c5.2xlarge"))
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

func handleCerts(certName string) (string, error) {
	err := os.Chmod(certName, 0o400)
	if err != nil {
		return "", err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	certFilePath := homeDir + "/.ssh/" + certName
	err = os.Rename(certName, certFilePath)
	if err != nil {
		return "", err
	}
	var stdBuffer bytes.Buffer
	cmd := exec.Command("ssh-add", certFilePath)
	mw := io.MultiWriter(os.Stdout, &stdBuffer)
	cmd.Stdout = mw
	cmd.Stderr = mw
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return certFilePath, nil
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
