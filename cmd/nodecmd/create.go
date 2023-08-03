// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/ava-labs/avalanche-cli/pkg/models"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	"github.com/ava-labs/avalanche-cli/pkg/terraform"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [clusterName]",
		Short: "Create a new validator on cloud server",
		Long: `The node create command sets up a validator on a cloud server of your choice. 
The validator will be validating the Avalanche Primary Network and Subnet 
of your choice. By default, the command runs an interactive wizard. It 
walks you through all the steps you need to set up a validator.
Once this command is completed, you will have to wait for the validator
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. You can check the bootstrapping
status by running avalanche node status`,
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
		keyPairExists, err := awsAPI.CheckKeyPairExists(ec2Svc, newKeyPairName)
		if err != nil {
			return "", err
		}
		if !keyPairExists {
			return newKeyPairName, nil
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Key Pair named %s already exists", newKeyPairName))
	}
}

func createNodeConfig(nodeID, region, ami, keyPairName, certPath, sg, eip, clusterName string) error {
	// eip and nodeID both are bounded by double quotation "", we need to remove them before they can be used
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
	clusterConfig := models.ClusterConfig{}
	var err error
	if app.ClusterConfigExists() {
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
	return app.UpdateClusterConfigFile(&clusterConfig)
}

func printNoCredentialsOutput() {
	ux.Logger.PrintToUser("No AWS credentials file found in ~/.aws/credentials")
	ux.Logger.PrintToUser("Create a file called 'credentials' with the contents below, and add the file to ~/.aws/ directory")
	ux.Logger.PrintToUser("===========BEGINNING OF FILE===========")
	ux.Logger.PrintToUser("[default]\naws_access_key_id=<AWS_ACCESS_KEY>\naws_secret_access_key=<AWS_SECRET_ACCESS_KEY>")
	ux.Logger.PrintToUser("===========END OF FILE===========")
	ux.Logger.PrintToUser("More info can be found at https://docs.aws.amazon.com/sdkref/latest/guide/file-format.html#file-format-creds")
}

func getAWSCloudCredentials(rootBody *hclwrite.Body, region string) (*session.Session, error) {
	creds := credentials.NewSharedCredentials("", constants.AWSDefaultCredential)
	_, err := creds.Get()
	if err != nil {
		printNoCredentialsOutput()
		return &session.Session{}, err
	}
	err = requestAWSAccountAuth()
	if err != nil {
		return &session.Session{}, err
	}
	err = terraform.SetCloudCredentials(rootBody, region)
	if err != nil {
		return &session.Session{}, err
	}
	// Load session from shared config
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	})
	if err != nil {
		return &session.Session{}, err
	}
	return sess, nil
}

func promptKeyPairName(ec2Svc *ec2.EC2, rootBody *hclwrite.Body, keyPairName string, keyPairExists bool) (string, string, error) {
	if keyPairExists {
		ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists in AWS", keyPairName))
		ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in your .ssh directory", keyPairName))
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists on your .ssh directory but not on AWS", keyPairName))
		ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in AWS", keyPairName))
	}
	newKeyPairName, err := getNewKeyPairName(ec2Svc)
	if err != nil {
		return "", "", err
	}
	certName := newKeyPairName + constants.CertSuffix
	terraform.SetKeyPair(rootBody, newKeyPairName, certName)
	return certName, newKeyPairName, nil
}

func createEC2Instance(rootBody *hclwrite.Body, hclFile *hclwrite.File, tfFile *os.File, region, certName, keyPairName, securityGroupName, ami string) (string, string, string, string, error) {
	sess, err := getAWSCloudCredentials(rootBody, region)
	if err != nil {
		return "", "", "", "", err
	}
	ux.Logger.PrintToUser("Creating a new EC2 instance on AWS...")
	ec2Svc := ec2.New(sess)
	var useExistingKeyPair bool
	certFilePath, err := app.GetSSHCertFilePath(certName)
	if err != nil {
		return "", "", "", "", err
	}
	keyPairExists, err := awsAPI.CheckKeyPairExists(ec2Svc, keyPairName)
	if err != nil {
		return "", "", "", "", err
	}
	if !keyPairExists {
		if !app.CheckCertInSSHDir(certFilePath) {
			ux.Logger.PrintToUser(fmt.Sprintf("Creating new key pair %s in AWS", keyPairName))
			terraform.SetKeyPair(rootBody, keyPairName, certName)
		} else {
			certName, keyPairName, err = promptKeyPairName(ec2Svc, rootBody, keyPairName, keyPairExists)
			if err != nil {
				return "", "", "", "", err
			}
		}
	} else {
		if app.CheckCertInSSHDir(certFilePath) {
			ux.Logger.PrintToUser(fmt.Sprintf("Using existing key pair %s in AWS", keyPairName))
			useExistingKeyPair = true
		} else {
			certName, keyPairName, err = promptKeyPairName(ec2Svc, rootBody, keyPairName, keyPairExists)
			if err != nil {
				return "", "", "", "", err
			}
		}
	}
	securityGroupExists, sg, err := awsAPI.CheckSecurityGroupExists(ec2Svc, securityGroupName)
	if err != nil {
		return "", "", "", "", err
	}
	userIPAddress, err := getIPAddress()
	if err != nil {
		return "", "", "", "", err
	}
	if !securityGroupExists {
		ux.Logger.PrintToUser(fmt.Sprintf("Creating new security group %s in AWS", securityGroupName))
		terraform.SetSecurityGroup(rootBody, userIPAddress, securityGroupName)
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("Using existing security group %s in AWS", securityGroupName))
		ipInTCP, ipInHTTP := awsAPI.CheckCurrentIPInSg(sg, userIPAddress)
		terraform.SetSecurityGroupRule(rootBody, userIPAddress, *sg.GroupId, ipInTCP, ipInHTTP)
	}
	terraform.SetElasticIP(rootBody)
	terraform.SetUpInstance(rootBody, securityGroupName, useExistingKeyPair, keyPairName, ami)
	terraform.SetOutput(rootBody)
	err = terraform.SaveTerraformFile(tfFile, hclFile)
	if err != nil {
		return "", "", "", "", err
	}
	err = app.CreateNodeTerraformDir()
	if err != nil {
		return "", "", "", "", err
	}
	instanceID, elasticIP, err := terraform.RunTerraform(app.GetNodeTerraformDir())
	if err != nil {
		return "", "", "", "", err
	}
	ux.Logger.PrintToUser("A new EC2 instance is successfully created in AWS!")
	certFilePath, err = app.GetSSHCertFilePath(certName)
	if err != nil {
		return "", "", "", "", err
	}

	if !useExistingKeyPair {
		err = addCertToSSH(certName)
		if err != nil {
			return "", "", "", "", err
		}
	}
	return instanceID, elasticIP, certFilePath, keyPairName, nil
}

func createNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := terraform.CheckIsInstalled(); err != nil {
		return err
	}
	if err := ansible.CheckIsInstalled(); err != nil {
		return err
	}
	err := terraform.RemoveExistingTerraformFiles(app.GetNodeTerraformDir())
	if err != nil {
		return err
	}
	usr, err := user.Current()
	if err != nil {
		return err
	}
	region := "us-east-2"
	ami := "ami-0430580de6244e02e"
	keyPairName := usr.Username + "-" + region + constants.AvalancheCLISuffix
	certName := keyPairName + "-" + region + constants.CertSuffix
	securityGroupName := keyPairName + "-" + region + constants.AWSSecurityGroupSuffix
	hclFile, tfFile, rootBody, err := terraform.CreateTerraformFile(app.GetNodeTerraformDir())
	if err != nil {
		return err
	}
	// Create new EC2 client
	instanceID, elasticIP, certFilePath, keyPairName, err := createEC2Instance(rootBody, hclFile, tfFile, region, certName, keyPairName, securityGroupName, ami)
	if err != nil {
		return err
	}
	err = terraform.RemoveExistingTerraformFiles(app.GetNodeTerraformDir())
	if err != nil {
		return err
	}
	inventoryPath := app.GetAnsibleInventoryPath(clusterName)
	if err := ansible.CreateAnsibleHostInventory(inventoryPath, elasticIP, certFilePath); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)

	ux.Logger.PrintToUser("Installing AvalancheGo and Avalanche-CLI and starting bootstrap process on the newly created EC2 instance...")
	if err := ansible.RunAnsibleSetUpNodePlaybook(inventoryPath); err != nil {
		return err
	}
	err = createNodeConfig(instanceID, region, ami, keyPairName, certFilePath, securityGroupName, elasticIP, clusterName)
	if err != nil {
		return err
	}
	PrintResults(instanceID, elasticIP, certFilePath, region)
	ux.Logger.PrintToUser("AvalancheGo and Avalanche-CLI installed and node is bootstrapping!")
	return nil
}

func requestAWSAccountAuth() error {
	confirm := "Do you authorize Avalanche-CLI to access your AWS account to set-up your Avalanche Validator node? " +
		"Please note that you will be charged for AWS usage."
	yes, err := app.Prompt.CaptureYesNo(confirm)
	if err != nil {
		return err
	}
	if !yes {
		return errors.New("user did not give authorization to Avalanche-CLI to access AWS account")
	}
	return nil
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

func addCertToSSH(certName string) error {
	err := os.Chmod(certName, 0o400)
	if err != nil {
		return err
	}
	certFilePath, err := app.GetSSHCertFilePath(certName)
	if err != nil {
		return err
	}
	err = os.Rename(certName, certFilePath)
	if err != nil {
		return err
	}
	cmd := exec.Command("ssh-add", certFilePath)
	utils.SetUpMultiWrite(cmd)
	return cmd.Run()
}

func PrintResults(instanceID, elasticIP, certFilePath, region string) {
	// eip and nodeID both are bounded by double quotation "", we need to remove them before they can be used
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
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(fmt.Sprintf("Don't delete or replace your ssh private key file at %s as you won't be able to access your cloud server without it", certFilePath))
}
