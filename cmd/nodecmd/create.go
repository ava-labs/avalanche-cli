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

	"github.com/ava-labs/avalanche-cli/pkg/models"

	terraform "github.com/ava-labs/avalanche-cli/pkg/terraform"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/cobra"
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

func getCertFilePath(certName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	certFilePath := homeDir + "/.ssh/" + certName
	return certFilePath, nil
}

func createNodeConfig(nodeID, region, ami, keyPairName, certPath, sg, eip, clusterName string) error {
	elasticIPToUse := eip[1 : len(eip)-2]
	nodeIDToUse := nodeID[1 : len(nodeID)-2]

	nodeConfig := models.NodeConfig{
		NodeID:        nodeIDToUse,
		Region:        region,
		AMI:           ami,
		KeyPair:       keyPairName,
		CertPath:      certPath,
		SecurityGroup: sg,
		ElasticIP:     elasticIPToUse,
	}
	err := app.CreateNodeConfigFile(nodeIDToUse, &nodeConfig)
	if err != nil {
		return err
	}
	return updateClusterConfig(nodeIDToUse, keyPairName, certPath, clusterName)
}

func updateClusterConfig(nodeID, keyPairName, certPath, clusterName string) error {
	configExists := app.ClusterConfigExists()
	clusterConfig := models.ClusterConfig{}
	var err error
	if configExists {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return err
		}
	}

	if clusterConfig.KeyPair == nil {
		clusterConfig.KeyPair = make(map[string]string)
	}
	if _, ok := clusterConfig.KeyPair[keyPairName]; !ok {
		clusterConfig.KeyPair[keyPairName] = certPath
	}
	if clusterConfig.Clusters == nil {
		clusterConfig.Clusters = make(map[string][]string)
	}
	if _, ok := clusterConfig.Clusters[clusterName]; !ok {
		clusterConfig.Clusters[clusterName] = []string{}
	}
	clusterConfig.Clusters[clusterName] = append(clusterConfig.Clusters[clusterName], nodeID)
	return app.UpdateClusterConfig(&clusterConfig)
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

	err := terraform.RemoveExistingTerraformFiles()
	if err != nil {
		return err
	}
	usr, err := user.Current()
	if err != nil {
		return err
	}
	keyPairName := usr.Username + "-avalanche-cli"
	certName := keyPairName + "-kp.pem"
	securityGroupName := keyPairName + "-sg"
	region := "us-east-2"
	ami := "ami-0430580de6244e02e"

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
	err = terraform.SetCloudCredentials(rootBody, credValue.AccessKeyID, credValue.SecretAccessKey, region)
	if err != nil {
		return err
	}
	// Load session from shared config
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	})
	if err != nil {
		return err
	}

	// Create new EC2 client
	ec2Svc := ec2.New(sess)
	var useExistingKeyPair bool
	keyPairExists, err := checkKeyPairExists(ec2Svc, keyPairName)
	if err != nil {
		return err
	}
	if !keyPairExists {
		terraform.SetKeyPair(rootBody, keyPairName, certName)
	} else {
		certFilePath, err := getCertFilePath(certName)
		if err != nil {
			return err
		}
		if checkCertInSSHDir(certFilePath) {
			useExistingKeyPair = true
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists", keyPairName))
			keyPairName, err = getNewKeyPairName(ec2Svc)
			if err != nil {
				return err
			}
			certName = keyPairName + "-kp.pem"
			terraform.SetKeyPair(rootBody, keyPairName, certName)
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
		terraform.SetSecurityGroup(rootBody, userIPAddress, securityGroupName)
	} else {
		ipInTCP, ipInHTTP := checkCurrentIPInSg(sg, userIPAddress)
		terraform.SetSecurityGroupRule(rootBody, userIPAddress, *sg.GroupId, ipInTCP, ipInHTTP)
	}
	terraform.SetElasticIP(rootBody)
	terraform.SetUpInstance(rootBody, securityGroupName, useExistingKeyPair, keyPairName, ami)
	terraform.SetOutput(rootBody)
	_, err = tfFile.Write(hclFile.Bytes())
	if err != nil {
		return err
	}
	instanceID, elasticIP, err := terraform.RunTerraform()
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
	err = createNodeConfig(instanceID, region, ami, keyPairName, certFilePath, securityGroupName, elasticIP, clusterName)
	if err != nil {
		return err
	}
	PrintResults(instanceID, elasticIP, certFilePath, region)
	err = terraform.RemoveExistingTerraformFiles()
	if err != nil {
		return err
	}
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

func checkCertInSSHDir(certFilePath string) bool {
	_, err := os.Stat(certFilePath)
	return err == nil
}

func checkCurrentIPInSg(sg *ec2.SecurityGroup, currentIP string) (bool, bool) {
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

func PrintResults(instanceID, elasticIP, certFilePath, region string) {
	instanceIDToUse := instanceID[1 : len(instanceID)-2]
	elasticIPToUse := elasticIP[1 : len(elasticIP)-2]
	ux.Logger.PrintToUser("VALIDATOR SUCCESSFULLY SET UP!")
	ux.Logger.PrintToUser("Please wait until validator is successfully boostrapped to run further commands on validator")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Here are the details of the set up validator: ")
	ux.Logger.PrintToUser(fmt.Sprintf("Cloud Instance ID: %s", instanceIDToUse))
	ux.Logger.PrintToUser(fmt.Sprintf("Elastic IP: %s", elasticIPToUse))
	ux.Logger.PrintToUser(fmt.Sprintf("Cloud Region: %s", region))
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("To ssh to validator, run: ")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(fmt.Sprintf("ssh -o IdentitiesOnly=yes ubuntu@%s -i %s", elasticIPToUse, certFilePath))
}
