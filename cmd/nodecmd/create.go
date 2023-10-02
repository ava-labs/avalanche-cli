// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	terraformGCP "github.com/ava-labs/avalanche-cli/pkg/terraform/gcp"
	"golang.org/x/exp/rand"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/vm"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/ava-labs/avalanche-cli/pkg/models"

	subnet "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/terraform"
	"github.com/ava-labs/avalanche-cli/pkg/terraform/aws"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/spf13/cobra"
)

type CloudConfig struct {
	InstanceIDs   []string
	PublicIPs     []string
	Region        string
	KeyPair       string
	SecurityGroup string
	CertFilePath  string
	ImageID       string
}

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
	cmd.Flags().BoolVar(&useEIP, "use-elastic-ip", true, "attach Elastic IP on AWS coud servers")

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
// func createClusterNodeConfig(nodeIDs, publicIPs []string, region, ami, keyPairName, certPath, sg, clusterName string) error {
func createClusterNodeConfig(cloudConfig CloudConfig, clusterName string) error {
	for i := range cloudConfig.InstanceIDs {
    publicIP := ""
		if len(cloudConfig.PublicIPs) > 0 {
			publicIP = cloudConfig.PublicIPs[i]
		}
		nodeConfig := models.NodeConfig{
			NodeID:        cloudConfig.InstanceIDs[i],
			Region:        cloudConfig.Region,
			AMI:           cloudConfig.ImageID,
			KeyPair:       cloudConfig.KeyPair,
			CertPath:      cloudConfig.CertFilePath,
			SecurityGroup: cloudConfig.SecurityGroup,
			ElasticIP:     publicIP,
		}
		err := app.CreateNodeCloudConfigFile(cloudConfig.InstanceIDs[i], &nodeConfig)
		if err != nil {
			return err
		}
		if err = addNodeToClusterConfig(cloudConfig.InstanceIDs[i], clusterName); err != nil {
			return err
		}
	}
	return updateKeyPairClusterConfig(cloudConfig)
}

func updateKeyPairClusterConfig(cloudConfig CloudConfig) error {
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
	if _, ok := clusterConfig.KeyPair[cloudConfig.KeyPair]; !ok {
		clusterConfig.KeyPair[cloudConfig.KeyPair] = cloudConfig.CertFilePath
	}
	return app.WriteClusterConfigFile(&clusterConfig)
}

func addNodeToClusterConfig(nodeID, clusterName string) error {
	clusterConfig := models.ClusterConfig{}
	var err error
	if app.ClusterConfigExists() {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return err
		}
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

func getServiceAccountKeyFilepath() (string, error) {
	ux.Logger.PrintToUser("To create a VM instance in GCP, we will need your service account credentials")
	ux.Logger.PrintToUser("Please follow instructions detailed at https://developers.google.com/workspace/guides/create-credentials#service-account to set up a GCP service account")
	ux.Logger.PrintToUser("Once completed, please enter the filepath to the JSON file containing the public/private key pair")
	ux.Logger.PrintToUser("For example: /Users/username/sample-project.json")
	return app.Prompt.CaptureString("What is the filepath to the credentials JSON file?")
}

func getGCPCloudCredentials() (*compute.Service, string, error) {
	var err error
	var gcpCredentialsPath string
	clusterConfig := models.ClusterConfig{}
	if app.ClusterConfigExists() {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return nil, "", err
		}
		gcpCredentialsPath = clusterConfig.ServiceAccountKeyFilepath
		if gcpCredentialsPath == "" {
			gcpCredentialsPath, err = getServiceAccountKeyFilepath()
			if err != nil {
				return nil, "", err
			}
		}
	} else {
		gcpCredentialsPath, err = getServiceAccountKeyFilepath()
		if err != nil {
			return nil, "", err
		}
	}
	os.Setenv(constants.GCPCredentialsEnvVar, gcpCredentialsPath)
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		return nil, "", err
	}
	computeService, err := compute.New(client)
	return computeService, gcpCredentialsPath, err
}

// getAWSCloudCredentials gets AWS account credentials defined in .aws dir in user home dir
func getAWSCloudCredentials(region, awsCommand string) (*session.Session, error) {
	if awsCommand == constants.StopAWSNode {
		if err := requestStopAWSNodeAuth(); err != nil {
			return &session.Session{}, err
		}
	} else if awsCommand == constants.CreateAWSNode {
		if err := requestAWSAccountAuth(); err != nil {
			return &session.Session{}, err
		}
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

func getGCPConfig() (*compute.Service, string, string, string, string, error) {
	usEast := "us-east1-b"
	usCentral := "us-central1-c"
	usWest := "us-west1-b"
	customRegion := "Choose custom zone (list of zones available at https://cloud.google.com/compute/docs/regions-zones)"
	zonePromptTxt := "Which GCP zone do you want to set up your node in?"
	zone, err := app.Prompt.CaptureList(
		zonePromptTxt,
		[]string{usEast, usCentral, usWest, customRegion},
	)
	if err != nil {
		return nil, "", "", "", "", err
	}
	if zone == customRegion {
		zone, err = app.Prompt.CaptureString(zonePromptTxt)
		if err != nil {
			return nil, "", "", "", "", err
		}
	}
	projectName, err := app.Prompt.CaptureString("What is the name of your Google Cloud project?")
	if err != nil {
		return nil, "", "", "", "", err
	}
	gcpClient, gcpCredentialFilePath, err := getGCPCloudCredentials()
	if err != nil {
		fmt.Printf("we have error here %s \n", err)
		return nil, "", "", "", "", err
	}
	imageID, err := gcpAPI.GetUbuntuImageID(gcpClient)
	if err != nil {
		return nil, "", "", "", "", err
	}
	return gcpClient, zone, imageID, gcpCredentialFilePath, projectName, nil
}

func getAWSCloudConfig() (*ec2.EC2, string, string, error) {
	usEast1 := "us-east-1"
	usEast2 := "us-east-2"
	usWest1 := "us-west-1"
	usWest2 := "us-west-2"
	customRegion := "Choose custom region (list of regions available at https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html)"
	region, err := app.Prompt.CaptureList(
		"Which AWS region do you want to set up your node in?",
		[]string{usEast1, usEast2, usWest1, usWest2, customRegion},
	)
	if err != nil {
		return nil, "", "", err
	}
	if region == customRegion {
		region, err = app.Prompt.CaptureString("Which AWS region do you want to set up your node in?")
		if err != nil {
			return nil, "", "", err
		}
	}
	sess, err := getAWSCloudCredentials(region, constants.CreateAWSNode)
	if err != nil {
		return nil, "", "", err
	}
	ec2Svc := ec2.New(sess)
	ami, err := awsAPI.GetUbuntuAMIID(ec2Svc)
	if err != nil {
		return nil, "", "", err
	}
	return ec2Svc, region, ami, nil
}

func randomString(length int) string {
	rand.Seed(uint64(time.Now().UnixNano()))
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

// createGCEInstances creates terraform .tf file and runs terraform exec function to create Google Compute Engine VM instances
func createGCEInstances(rootBody *hclwrite.Body,
	gcpClient *compute.Service,
	hclFile *hclwrite.File,
	zone,
	ami,
	cliDefaultName,
	projectName,
	credentialsPath string,
) ([]string, []string, string, string, error) {
	keyPairName := fmt.Sprintf("%s-keypair", cliDefaultName)
	sshKeyPath, err := app.GetSSHCertFilePath(keyPairName)
	networkName := fmt.Sprintf("%s-network", cliDefaultName)
	if err := terraformGCP.SetCloudCredentials(rootBody, zone, credentialsPath, projectName); err != nil {
		return nil, nil, "", "", err
	}
	numNodes, err := app.Prompt.CaptureUint32("How many nodes do you want to set up on GCP?")
	if err != nil {
		return nil, nil, "", "", err
	}
	ux.Logger.PrintToUser("Creating new VM instance(s) on Google Compute Engine...")
	certInSSHDir, err := app.CheckCertInSSHDir(fmt.Sprintf("%s-keypair.pub", cliDefaultName))
	if err != nil {
		return nil, nil, "", "", err
	}
	if !certInSSHDir {
		ux.Logger.PrintToUser("Creating new SSH key pair %s in GCP", sshKeyPath)
		ux.Logger.PrintToUser("For more information regarding SSH key pair in GCP, please head to https://cloud.google.com/compute/docs/connect/create-ssh-keys")
		_, err = exec.Command("ssh-keygen", "-t", "rsa", "-f", sshKeyPath, "-C", keyPairName, "-b", "2048").Output()
		if err != nil {
			return nil, nil, "", "", err
		}
	}

	networkExists, err := gcpAPI.CheckNetworkExists(gcpClient, projectName, networkName)
	if err != nil {
		return nil, nil, "", "", err
	}
	userIPAddress, err := getIPAddress()
	if err != nil {
		return nil, nil, "", "", err
	}
	if !networkExists {
		ux.Logger.PrintToUser(fmt.Sprintf("Creating new network %s in GCP", networkName))
		terraformGCP.SetNetwork(rootBody, userIPAddress, networkName)
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("Using existing network %s in GCP", networkName))
		firewallName := fmt.Sprintf("%s-%s", networkName, strings.ReplaceAll(userIPAddress, ".", ""))
		firewallExists, err := gcpAPI.CheckFirewallExists(gcpClient, projectName, firewallName)
		if err != nil {
			return nil, nil, "", "", err
		}
		if !firewallExists {
			terraformGCP.SetFirewallRule(rootBody, userIPAddress+"/32", firewallName, networkName, []string{strconv.Itoa(constants.SSHTCPPort), strconv.Itoa(constants.AvalanchegoAPIPort)})
		}
	}
	nodeName := fmt.Sprintf("gcp-node-%s", randomString(5))
	publicIPName := fmt.Sprintf("static-ip-%s", nodeName)
	terraformGCP.SetPublicIP(rootBody, nodeName, numNodes)
	sshPublicKey, err := os.ReadFile(fmt.Sprintf("%s.pub", sshKeyPath))
	if err != nil {
		return nil, nil, "", "", err
	}
	terraformGCP.SetupInstances(rootBody, networkName, string(sshPublicKey), ami, publicIPName, nodeName, keyPairName, numNodes)
	terraformGCP.SetOutput(rootBody)
	err = app.CreateTerraformDir()
	if err != nil {
		return nil, nil, "", "", err
	}
	err = terraform.SaveConf(app.GetTerraformDir(), hclFile)
	if err != nil {
		return nil, nil, "", "", err
	}
	//instanceIDs, elasticIPs, err := terraformGCP.RunTerraform(app.GetTerraformDir())
	_, _, err = terraformGCP.RunTerraform(app.GetTerraformDir())
	if err != nil {
		return nil, nil, "", "", err
	}
	ux.Logger.PrintToUser("New VM instance(s) successfully created in Google Cloud Engine!")
	//if !useExistingKeyPair {
	//	// takes the cert file downloaded from AWS through terraform and moves it to .ssh directory
	//	err = addCertToSSH(certName)
	//	if err != nil {
	//		return nil, nil, "", "", err
	//	}
	//}
	//sshCertPath, err := app.GetSSHCertFilePath(certName)
	//if err != nil {
	//	return nil, nil, "", "", err
	//}
	//return instanceIDs, elasticIPs, sshCertPath, keyPairName, nil
	return nil, nil, "", "", nil
}

// createEC2Instances creates terraform .tf file and runs terraform exec function to create ec2 instances
func createEC2Instances(rootBody *hclwrite.Body,
	ec2Svc *ec2.EC2,
	hclFile *hclwrite.File,
	region,
	ami,
	certName,
	keyPairName,
	securityGroupName string,
) ([]string, []string, string, string, error) {
	if err := terraformAWS.SetCloudCredentials(rootBody, region); err != nil {
		return nil, nil, "", "", err
	}
	numNodes, err := app.Prompt.CaptureUint32("How many nodes do you want to set up on AWS?")
	if err != nil {
		return nil, nil, "", "", err
	}
	ux.Logger.PrintToUser("Creating new EC2 instance(s) on AWS...")
	var useExistingKeyPair bool
	keyPairExists, err := awsAPI.CheckKeyPairExists(ec2Svc, keyPairName)
	if err != nil {
		return nil, nil, "", "", err
	}
	certInSSHDir, err := app.CheckCertInSSHDir(certName)
	if err != nil {
		return nil, nil, "", "", err
	}
	if !keyPairExists {
		if !certInSSHDir {
			ux.Logger.PrintToUser(fmt.Sprintf("Creating new key pair %s in AWS", keyPairName))
			terraformAWS.SetKeyPair(rootBody, keyPairName, certName)
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists on your .ssh directory but not on AWS", keyPairName))
			ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in AWS", keyPairName))
			certName, keyPairName, err = promptKeyPairName(ec2Svc)
			if err != nil {
				return nil, nil, "", "", err
			}
			terraformAWS.SetKeyPair(rootBody, keyPairName, certName)
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
				return nil, nil, "", "", err
			}
			terraformAWS.SetKeyPair(rootBody, keyPairName, certName)
		}
	}
	securityGroupExists, sg, err := awsAPI.CheckSecurityGroupExists(ec2Svc, securityGroupName)
	if err != nil {
		return nil, nil, "", "", err
	}
	userIPAddress, err := getIPAddress()
	if err != nil {
		return nil, nil, "", "", err
	}
	if !securityGroupExists {
		ux.Logger.PrintToUser(fmt.Sprintf("Creating new security group %s in AWS", securityGroupName))
		terraformAWS.SetSecurityGroup(rootBody, userIPAddress, securityGroupName)
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("Using existing security group %s in AWS", securityGroupName))
		ipInTCP := awsAPI.CheckUserIPInSg(sg, userIPAddress, constants.SSHTCPPort)
		ipInHTTP := awsAPI.CheckUserIPInSg(sg, userIPAddress, constants.AvalanchegoAPIPort)
		terraformAWS.SetSecurityGroupRule(rootBody, userIPAddress, *sg.GroupId, ipInTCP, ipInHTTP)
	}
	if useEIP {
		terraform.SetElasticIPs(rootBody, numNodes)
	}
	terraform.SetupInstances(rootBody, securityGroupName, useExistingKeyPair, keyPairName, ami, numNodes)
	terraform.SetOutput(rootBody, useEIP)
	err = app.CreateTerraformDir()
	if err != nil {
		return nil, nil, "", "", err
	}
	err = terraform.SaveConf(app.GetTerraformDir(), hclFile)
	if err != nil {
		return nil, nil, "", "", err
	}
	instanceIDs, elasticIPs, err := terraform.RunTerraform(app.GetTerraformDir(), useEIP)
	if err != nil {
		return nil, nil, "", "", err
	}
	ux.Logger.PrintToUser("New EC2 instance(s) successfully created in AWS!")
	if !useExistingKeyPair {
		// takes the cert file downloaded from AWS through terraform and moves it to .ssh directory
		err = addCertToSSH(certName)
		if err != nil {
			return nil, nil, "", "", err
		}
	}
	sshCertPath, err := app.GetSSHCertFilePath(certName)
	if err != nil {
		return nil, nil, "", "", err
	}
	return instanceIDs, elasticIPs, sshCertPath, keyPairName, nil
}

func createAWSInstance(usr *user.User) (CloudConfig, error) {
	// Get AWS Credential, region and AMI
	ec2Svc, region, ami, err := getAWSCloudConfig()
	if err != nil {
		return CloudConfig{}, nil
	}
	prefix := usr.Username + "-" + region + constants.AvalancheCLISuffix
	certName := prefix + "-" + region + constants.CertSuffix
	securityGroupName := prefix + "-" + region + constants.AWSSecurityGroupSuffix
	hclFile, rootBody, err := terraform.InitConf()
	if err != nil {
		return CloudConfig{}, nil
	}

	// Create new EC2 instances
	instanceIDs, elasticIPs, certFilePath, keyPairName, err := createEC2Instances(rootBody, ec2Svc, hclFile, region, ami, certName, prefix, securityGroupName)
	if err != nil {
		if err.Error() == constants.EIPLimitErr {
			ux.Logger.PrintToUser("Failed to create AWS cloud server, please try creating again in a different region")
		} else {
			ux.Logger.PrintToUser("Failed to create AWS cloud server")
		}
		// we stop created instances so that user doesn't pay for unused EC2 instances
		instanceIDs, instanceIDErr := terraformAWS.GetInstanceIDs(app.GetTerraformDir())
		if instanceIDErr != nil {
			return CloudConfig{}, instanceIDErr
		}
		failedNodes := []string{}
		nodeErrors := []error{}
		for _, instanceID := range instanceIDs {
			ux.Logger.PrintToUser(fmt.Sprintf("Stopping AWS cloud server %s...", instanceID))
			if stopErr := awsAPI.StopInstance(ec2Svc, instanceID, "", false); stopErr != nil {
				failedNodes = append(failedNodes, instanceID)
				nodeErrors = append(nodeErrors, stopErr)
			}
			ux.Logger.PrintToUser(fmt.Sprintf("AWS cloud server instance %s stopped", instanceID))
		}
		if len(failedNodes) > 0 {
			ux.Logger.PrintToUser("Failed nodes: ")
			for i, node := range failedNodes {
				ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, nodeErrors[i]))
			}
			ux.Logger.PrintToUser("Stop the above instance(s) on AWS console to prevent charges")
			return CloudConfig{}, fmt.Errorf("failed to stop node(s) %s", failedNodes)
		}
		return CloudConfig{}, nil
	}
	awsCloudConfig := CloudConfig{
		instanceIDs,
		elasticIPs,
		region,
		keyPairName,
		securityGroupName,
		certFilePath,
		ami,
	}
	return awsCloudConfig, nil
}

func createGCPInstance(usr *user.User) (CloudConfig, error) {
	// Get GCP Credential, zone, Image ID, service account key file path, and GCP project name
	gcpClient, zone, imageID, gcpCredentialFilepath, gcpProjectName, err := getGCPConfig()
	if err != nil {
		return CloudConfig{}, nil
	}
	defaultAvalancheCLIPrefix := usr.Username + constants.AvalancheCLISuffix
	hclFile, rootBody, err := terraform.InitConf()
	if err != nil {
		return CloudConfig{}, nil
	}
	//instanceIDs, elasticIPs, certFilePath, keyPairName, err := createGCEInstances(rootBody, gcpClient, hclFile, zone, imageID, defaultAvalancheCLIPrefix, gcpProjectName, "", gcpCredentialFilepath)
	_, _, _, _, err = createGCEInstances(rootBody, gcpClient, hclFile, zone, imageID, defaultAvalancheCLIPrefix, gcpProjectName, gcpCredentialFilepath)
	if err != nil {
		return CloudConfig{}, nil
	}
	//gcpCloudConfig := CloudConfig{
	//	instanceIDs,
	//	elasticIPs,
	//	region,
	//	keyPairName,
	//	firewallName,
	//	certFilePath,
	//	ami,
	//}
	//return gcpCloudConfig, nil
	return CloudConfig{}, nil
}

func createNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	cloudService, err := promptCloudService()
	if err != nil {
		return err
	}
	if err := terraform.CheckIsInstalled(); err != nil {
		return err
	}
	if err := ansible.CheckIsInstalled(); err != nil {
		return err
	}
	err = terraform.RemoveDirectory(app.GetTerraformDir())
	if err != nil {
		return err
	}
	usr, err := user.Current()
	if err != nil {
		return err
	}
	cloudConfig := CloudConfig{}
	if cloudService == constants.AWSCloudService {
		cloudConfig, err = createAWSInstance(usr)
	} else {
		cloudConfig, err = createGCPInstance(usr)
	}
	_, err = createGCPInstance(usr)
	if err != nil {
		return err
	}
	if err := createClusterNodeConfig(cloudConfig, clusterName); err != nil {
		return err
	}
	err = terraform.RemoveDirectory(app.GetTerraformDir())
	if err != nil {
		return err
	}
	publicIPMap := map[string]string{}
	if !useEIP {
		publicIPMap, err = awsAPI.GetInstancePublicIPs(ec2Svc, instanceIDs)
		if err != nil {
			return err
		}
	} else {
		for i, node := range instanceIDs {
			publicIPMap[node] = elasticIPs[i]
		}
	}
	inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	if err := ansible.CreateAnsibleHostInventory(inventoryPath, cloudConfig.CertFilePath, cloudConfig.PublicIPs, cloudConfig.InstanceIDs); err != nil {
		return err
	}
	time.Sleep(15 * time.Second)

	avalancheGoVersion, err := getAvalancheGoVersion()
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Installing AvalancheGo and Avalanche-CLI and starting bootstrap process on the newly created Avalanche node(s) ...")
	if err := runAnsible(inventoryPath, avalancheGoVersion); err != nil {
		return err
	}
	if err := setupBuildEnv(clusterName); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Copying staker.crt and staker.key to local machine...")
	for _, instanceID := range cloudConfig.InstanceIDs {
		nodeInstanceDirPath := app.GetNodeInstanceDirPath(instanceID)
		// ansible host alias's name is formatted as ansiblePrefix_{instanceID}
		ansiblePrefix := constants.AWSNodeAnsiblePrefix
		if cloudService == constants.GCPCloudService {
			ansiblePrefix = constants.GCPNodeAnsiblePrefix
		}
		nodeInstanceAnsibleAlias := fmt.Sprintf("%s_%s", ansiblePrefix, instanceID)
		if err := ansible.RunAnsiblePlaybookCopyStakingFiles(app.GetAnsibleDir(), nodeInstanceAnsibleAlias, nodeInstanceDirPath, inventoryPath); err != nil {
			return err
		}
	}
	PrintResults(cloudConfig)
	ux.Logger.PrintToUser("AvalancheGo and Avalanche-CLI installed and node(s) are bootstrapping!")
	return nil
}

// setupAnsible we need to remove existing ansible directory and its contents in .avalanche-cli dir
// before calling every ansible run command just in case there is a change in playbook
func setupAnsible(clusterName string) error {
	err := app.SetupAnsibleEnv()
	if err != nil {
		return err
	}
	if err = ansible.Setup(app.GetAnsibleDir()); err != nil {
		return err
	}
	return updateAnsiblePublicIPs(clusterName)
}

func runAnsible(inventoryPath, avalancheGoVersion, clusterName string) error {
	err := setupAnsible(clusterName)
	if err != nil {
		return err
	}
	return ansible.RunAnsiblePlaybookSetupNode(app.GetConfigPath(), app.GetAnsibleDir(), inventoryPath, avalancheGoVersion)
}

func setupBuildEnv(clusterName string) error {
	ux.Logger.PrintToUser("Installing Custom VM build environment on the EC2 instance(s) ...")
	inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	if err := ansible.RunAnsiblePlaybookSetupBuildEnv(app.GetAnsibleDir(), inventoryPath, "all"); err != nil {
		return err
	}
	return nil
}

func requestAWSAccountAuth() error {
	ux.Logger.PrintToUser("Do you authorize Avalanche-CLI to access your AWS account to set-up your Avalanche Validator node?")
	ux.Logger.PrintToUser("Please note that you will be charged for AWS usage.")
	ux.Logger.PrintToUser("By clicking yes, you are authorizing Avalanche-CLI to:")
	ux.Logger.PrintToUser("- Set up EC2 instance(s) and other components (such as security groups, key pairs and elastic IPs)")
	ux.Logger.PrintToUser("- Set up the EC2 instance(s) to validate the Avalanche Primary Network")
	ux.Logger.PrintToUser("- Set up the EC2 instance(s) to validate Subnets")
	yes, err := app.Prompt.CaptureYesNo("I authorize Avalanche-CLI to access my AWS account")
	if err != nil {
		return err
	}
	if !yes {
		return errors.New("user did not give authorization to Avalanche-CLI to access AWS account")
	}
	return nil
}

func requestStopAWSNodeAuth() error {
	ux.Logger.PrintToUser("Do you authorize Avalanche-CLI to access your AWS account to stop your Avalanche Validator node?")
	ux.Logger.PrintToUser("By clicking yes, you are authorizing Avalanche-CLI to:")
	ux.Logger.PrintToUser("- Stop EC2 instance(s) and other components (such as elastic IPs)")
	yes, err := app.Prompt.CaptureYesNo("I authorize Avalanche-CLI to access my AWS account")
	if err != nil {
		return err
	}
	if !yes {
		return errors.New("user did not give authorization to Avalanche-CLI to access AWS account")
	}
	return nil
}

func getIPAddress() (string, error) {
	ipOutput, err := exec.Command("curl", "https://api.ipify.org?format=json").Output()
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	if err = json.Unmarshal(ipOutput, &result); err != nil {
		return "", err
	}
	ipAddress, ok := result["ip"].(string)
	if ok {
		if net.ParseIP(ipAddress) == nil {
			return "", errors.New("invalid IP address")
		}
		return ipAddress, nil
	}
	return "", errors.New("no IP address found")
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
	utils.SetupRealtimeCLIOutput(cmd, true, true)
	return cmd.Run()
}

// getAvalancheGoVersion asks users whether they want to install the newest Avalanche Go version
// or if they want to use the newest Avalanche Go Version that is still compatible with Subnet EVM
// version of their choice
func getAvalancheGoVersion() (string, error) {
	chosenOption, err := promptAvalancheGoReferenceChoice()
	if err != nil {
		return "", err
	}
	if chosenOption != "latest" {
		sc, err := app.LoadSidecar(chosenOption)
		if err != nil {
			return "", err
		}
		customAvagoVersion, err := GetLatestAvagoVersionForRPC(sc.RPCVersion)
		if err != nil {
			return "", err
		}
		return customAvagoVersion, nil
	}
	return chosenOption, nil
}

func GetLatestAvagoVersionForRPC(configuredRPCVersion int) (string, error) {
	desiredAvagoVersion, err := vm.GetLatestAvalancheGoByProtocolVersion(
		app, configuredRPCVersion, constants.AvalancheGoCompatibilityURL)
	if err != nil {
		return "", err
	}
	return desiredAvagoVersion, nil
}

// promptAvalancheGoReferenceChoice returns user's choice of either using the latest Avalanche Go
// version or using the latest Avalanche Go version that is still compatible with the subnet that user
// wants the cloud server to track
func promptAvalancheGoReferenceChoice() (string, error) {
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

func promptCloudService() (string, error) {
	txt := "Which cloud service would you like to launch your Avalanche Node in?"
	cloudOptions := []string{constants.AWSCloudService, constants.GCPCloudService}
	chosenCloudService, err := app.Prompt.CaptureList(txt, cloudOptions)
	if err != nil {
		return "", err
	}
	return chosenCloudService, nil
}

func PrintResults(cloudConfig CloudConfig) {
	ux.Logger.PrintToUser("======================================")
	ux.Logger.PrintToUser("AVALANCHE NODE(S) SUCCESSFULLY SET UP!")
	ux.Logger.PrintToUser("======================================")
	ux.Logger.PrintToUser("Please wait until the node(s) are successfully bootstrapped to run further commands on the node(s)")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Here are the details of the set up node(s): ")
	for i, instanceID := range cloudConfig.InstanceIDs {
		ux.Logger.PrintToUser("======================================")
		hostAliasName := fmt.Sprintf("aws_node_%s", cloudConfig.InstanceIDs[i])
		ux.Logger.PrintToUser(fmt.Sprintf("Node %s details: ", hostAliasName))
		ux.Logger.PrintToUser(fmt.Sprintf("Cloud Instance ID: %s", instanceID))
		ux.Logger.PrintToUser(fmt.Sprintf("Elastic IP: %s", cloudConfig.PublicIPs[i]))
		ux.Logger.PrintToUser(fmt.Sprintf("Cloud Region: %s", cloudConfig.Region))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(fmt.Sprintf("staker.crt and staker.key are stored at %s. If anything happens to your node or the machine node runs on, these files can be used to fully recreate your node.", app.GetNodeInstanceDirPath(instanceID)))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("To ssh to node, run: ")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(fmt.Sprintf("ssh -o IdentitiesOnly=yes ubuntu@%s -i %s", cloudConfig.PublicIPs[i], cloudConfig.CertFilePath))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("======================================")
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Don't delete or replace your ssh private key file at %s as you won't be able to access your cloud server without it", cloudConfig.CertFilePath))
	ux.Logger.PrintToUser("")
}
