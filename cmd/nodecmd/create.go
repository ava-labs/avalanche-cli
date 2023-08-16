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

	"github.com/ava-labs/avalanche-cli/pkg/localnetworkinterface"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/ava-labs/avalanche-cli/pkg/models"

	subnet "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
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
		Short: "(ALPHA Warning) Create a new validator on cloud server",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node create command sets up a validator on a cloud server of your choice. 
The validator will be validating the Avalanche Primary Network and Subnet 
of your choice. By default, the command runs an interactive wizard. It 
walks you through all the steps you need to set up a validator.
Once this command is completed, you will have to wait for the validator
to finish bootstrapping on the primary network before running further
commands on it, e.g. validating a Subnet. You can check the bootstrapping
status by running avalanche node status 

The created node will be part of group of validators called <clusterName> 
and users can call node commands with <clusterName> so that the command
will apply to all nodes in the cluster`,
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

// createClusterNodeConfig creates node config and save it in .avalanche-cli/nodes/{instanceID}
// also creates cluster config in .avalanche-cli/nodes storing various key pair and security group info for all clusters
func createClusterNodeConfig(nodeID, region, ami, keyPairName, certPath, sg, eip, clusterName string) error {
	nodeConfig := models.NodeConfig{
		NodeID:        nodeID,
		Region:        region,
		AMI:           ami,
		KeyPair:       keyPairName,
		CertPath:      certPath,
		SecurityGroup: sg,
		ElasticIP:     eip,
	}
	err := app.CreateNodeCloudConfigFile(nodeID, &nodeConfig)
	if err != nil {
		return err
	}
	return updateClusterConfig(nodeID, keyPairName, certPath, clusterName)
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
	return app.WriteClusterConfigFile(&clusterConfig)
}

func printNoCredentialsOutput() {
	ux.Logger.PrintToUser("No AWS credentials file found in ~/.aws/credentials")
	ux.Logger.PrintToUser("Create a file called 'credentials' with the contents below, and add the file to ~/.aws/ directory")
	ux.Logger.PrintToUser("===========BEGINNING OF FILE===========")
	ux.Logger.PrintToUser("[default]\naws_access_key_id=<AWS_ACCESS_KEY>\naws_secret_access_key=<AWS_SECRET_ACCESS_KEY>")
	ux.Logger.PrintToUser("===========END OF FILE===========")
	ux.Logger.PrintToUser("More info can be found at https://docs.aws.amazon.com/sdkref/latest/guide/file-format.html#file-format-creds")
}

// getAWSCloudCredentials gets AWS account credentials defined in .aws dir in user home dir
func getAWSCloudCredentials(region string) (*session.Session, error) {
	if err := requestAWSAccountAuth(); err != nil {
		return &session.Session{}, err
	}
	creds := credentials.NewSharedCredentials("", constants.AWSDefaultCredential)
	if _, err := creds.Get(); err != nil {
		printNoCredentialsOutput()
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

// promptKeyPairName get custom name for key pair if the default key pair name that we use cannot be used for this EC2 instance
func promptKeyPairName(ec2Svc *ec2.EC2) (string, string, error) {
	newKeyPairName, err := getNewKeyPairName(ec2Svc)
	if err != nil {
		return "", "", err
	}
	certName := newKeyPairName + constants.CertSuffix
	return certName, newKeyPairName, nil
}

// createEC2Instance creates terraform .tf file and runs terraform exec function to create ec2 instance
func createEC2Instance(rootBody *hclwrite.Body,
	hclFile *hclwrite.File,
	region,
	certName,
	keyPairName,
	securityGroupName,
	ami string,
) (string, string, string, string, error) {
	sess, err := getAWSCloudCredentials(region)
	if err != nil {
		return "", "", "", "", err
	}
	if err := terraform.SetCloudCredentials(rootBody, region); err != nil {
		return "", "", "", "", err
	}
	ux.Logger.PrintToUser("Creating a new EC2 instance on AWS...")
	ec2Svc := ec2.New(sess)
	var useExistingKeyPair bool
	keyPairExists, err := awsAPI.CheckKeyPairExists(ec2Svc, keyPairName)
	if err != nil {
		return "", "", "", "", err
	}
	certInSSHDir, err := app.CheckCertInSSHDir(certName)
	if err != nil {
		return "", "", "", "", err
	}
	if !keyPairExists {
		if !certInSSHDir {
			ux.Logger.PrintToUser(fmt.Sprintf("Creating new key pair %s in AWS", keyPairName))
			terraform.SetKeyPair(rootBody, keyPairName, certName)
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists on your .ssh directory but not on AWS", keyPairName))
			ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in AWS", keyPairName))
			certName, keyPairName, err = promptKeyPairName(ec2Svc)
			if err != nil {
				return "", "", "", "", err
			}
			terraform.SetKeyPair(rootBody, keyPairName, certName)
		}
	} else {
		if certInSSHDir {
			ux.Logger.PrintToUser(fmt.Sprintf("Using existing key pair %s in AWS", keyPairName))
			useExistingKeyPair = true
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists in AWS", keyPairName))
			ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in your .ssh directory", keyPairName))
			certName, keyPairName, err = promptKeyPairName(ec2Svc)
			if err != nil {
				return "", "", "", "", err
			}
			terraform.SetKeyPair(rootBody, keyPairName, certName)
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
		ipInTCP := awsAPI.CheckUserIPInSg(sg, userIPAddress, constants.SSHTCPPort)
		ipInHTTP := awsAPI.CheckUserIPInSg(sg, userIPAddress, constants.AvalanchegoAPIPort)
		terraform.SetSecurityGroupRule(rootBody, userIPAddress, *sg.GroupId, ipInTCP, ipInHTTP)
	}
	terraform.SetElasticIP(rootBody)
	terraform.SetupInstance(rootBody, securityGroupName, useExistingKeyPair, keyPairName, ami)
	terraform.SetOutput(rootBody)
	err = app.CreateTerraformDir()
	if err != nil {
		return "", "", "", "", err
	}
	err = terraform.SaveConf(app.GetTerraformDir(), hclFile)
	if err != nil {
		fmt.Printf("error here ")
		return "", "", "", "", err
	}
	instanceID, elasticIP, err := terraform.RunTerraform(app.GetTerraformDir())
	if err != nil {
		return "", "", "", "", err
	}
	ux.Logger.PrintToUser("A new EC2 instance is successfully created in AWS!")
	if !useExistingKeyPair {
		// takes the cert file downloaded from AWS through terraform and moves it to .ssh directory
		err = addCertToSSH(certName)
		if err != nil {
			return "", "", "", "", err
		}
	}
	sshCertPath, err := app.GetSSHCertFilePath(certName)
	if err != nil {
		return "", "", "", "", err
	}
	return instanceID, elasticIP, sshCertPath, keyPairName, nil
}

func createNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := terraform.CheckIsInstalled(); err != nil {
		return err
	}
	if err := ansible.CheckIsInstalled(); err != nil {
		return err
	}
	err := terraform.RemoveDirectory(app.GetTerraformDir())
	if err != nil {
		return err
	}
	usr, err := user.Current()
	if err != nil {
		return err
	}
	region := "us-east-2"
	ami := "ami-0430580de6244e02e"
	prefix := usr.Username + "-" + region + constants.AvalancheCLISuffix
	certName := prefix + "-" + region + constants.CertSuffix
	securityGroupName := prefix + "-" + region + constants.AWSSecurityGroupSuffix
	hclFile, rootBody, err := terraform.InitConf()
	if err != nil {
		return err
	}
	// Create new EC2 client
	instanceID, elasticIP, certFilePath, keyPairName, err := createEC2Instance(rootBody, hclFile, region, certName, prefix, securityGroupName, ami)
	if err != nil {
		return err
	}
	err = terraform.RemoveDirectory(app.GetTerraformDir())
	if err != nil {
		return err
	}
	inventoryPath := app.GetAnsibleInventoryPath(clusterName)
	if err := ansible.CreateAnsibleHostInventory(inventoryPath, elasticIP, certFilePath); err != nil {
		return err
	}
	time.Sleep(15 * time.Second)

	avalancheGoVersion, err := getAvalancheGoVersion()
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Installing AvalancheGo and Avalanche-CLI and starting bootstrap process on the newly created EC2 instance...")
	if err := runAnsible(inventoryPath, avalancheGoVersion); err != nil {
		return err
	}
	err = createClusterNodeConfig(instanceID, region, ami, keyPairName, certFilePath, securityGroupName, elasticIP, clusterName)
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Copying staker.crt and staker.key to local machine...")
	if err := ansible.RunAnsibleCopyStakingFilesPlaybook(app.GetAnsibleDir(), app.GetNodeInstanceDirPath(instanceID), inventoryPath); err != nil {
		return err
	}
	PrintResults(instanceID, elasticIP, certFilePath, region)
	ux.Logger.PrintToUser("AvalancheGo and Avalanche-CLI installed and node is bootstrapping!")
	return nil
}

// setupAnsible we need to remove existing ansible directory and its contents in .avalanche-cli dir
// before calling every ansible run command just in case there is a change in playbook
func setupAnsible() error {
	err := app.SetupAnsibleEnv()
	if err != nil {
		return err
	}
	return ansible.Setup(app.GetAnsibleDir())
}

func runAnsible(inventoryPath, avalancheGoVersion string) error {
	err := setupAnsible()
	if err != nil {
		return err
	}
	return ansible.RunAnsibleSetupNodePlaybook(app.GetConfigPath(), app.GetAnsibleDir(), inventoryPath, avalancheGoVersion)
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

// addCertToSSH takes the cert file downloaded from AWS through terraform and moves it to .ssh directory
func addCertToSSH(certName string) error {
	certPath := app.GetTempCertPath(certName)
	err := os.Chmod(certPath, 0o400)
	if err != nil {
		return err
	}
	certFilePath, err := app.GetSSHCertFilePath(certName)
	if err != nil {
		return err
	}
	err = os.Rename(certPath, certFilePath)
	if err != nil {
		return err
	}
	cmd := exec.Command("ssh-add", certFilePath)
	utils.SetupRealtimeCLIOutput(cmd)
	return cmd.Run()
}

// getAvalancheGoVersion asks users whether they want to install the newest Avalanche Go version
// or if they want to use the newest AValanche Go Version that is still compatible with Subnet EVM
// version of their choice
func getAvalancheGoVersion() (string, error) {
	chosenOption, err := promptAvalancheGoVersion()
	if err != nil {
		return "", err
	}
	if chosenOption != "latest" {
		sc, err := app.LoadSidecar(chosenOption)
		if err != nil {
			return "", err
		}
		nc := localnetworkinterface.NewStatusChecker()
		customAvagoVersion, err := subnet.CheckForInvalidDeployAndGetAvagoVersion(nc, sc.RPCVersion)
		if err != nil {
			return "", err
		}
		return customAvagoVersion, nil
	}
	return chosenOption, nil
}

// promptAvalancheGoVersion either returns latest or the subnet that user wants to use as avago version reference
func promptAvalancheGoVersion() (string, error) {
	defaultVersion := "Use latest Avalanche Go Version"
	txt := "What version of Avalanche Go would you like to install in the node?"
	versionOptions := []string{defaultVersion, "Use the deployed Subnet's VM version that the node will be validating"}
	versionOption, err := app.Prompt.CaptureList(txt, versionOptions)
	if err != nil {
		return "", err
	}

	switch versionOption {
	case defaultVersion:
		return "latest", nil
	default:
		for {
			subnetName, err := app.Prompt.CaptureString("Which Subnet would you like to use to choose the avalanche go version?")
			if err != nil {
				return "", err
			}
			_, err = subnet.ValidateSubnetNameAndGetChains([]string{subnetName})
			if err == nil {
				return subnetName, nil
			}
			ux.Logger.PrintToUser(fmt.Sprintf("no subnet named %s found", subnetName))
		}
	}
}

func PrintResults(instanceID, elasticIP, certFilePath, region string) {
	ux.Logger.PrintToUser("VALIDATOR SUCCESSFULLY SET UP!")
	ux.Logger.PrintToUser("Please wait until validator is successfully boostrapped to run further commands on validator")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Here are the details of the set up validator: ")
	ux.Logger.PrintToUser(fmt.Sprintf("Cloud Instance ID: %s", instanceID))
	ux.Logger.PrintToUser(fmt.Sprintf("Elastic IP: %s", elasticIP))
	ux.Logger.PrintToUser(fmt.Sprintf("Cloud Region: %s", region))
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(fmt.Sprintf("Don't delete or replace your ssh private key file at %s as you won't be able to access your cloud server without it", certFilePath))
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(fmt.Sprintf("staker.crt and staker.key are stored at %s. If anything happens to your node or the machine node runs on, these files can be used to fully recreate your node.", app.GetNodeInstanceDirPath(instanceID)))
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("To ssh to validator, run: ")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser(fmt.Sprintf("ssh -o IdentitiesOnly=yes ubuntu@%s -i %s", elasticIP, certFilePath))
	ux.Logger.PrintToUser("")
}
