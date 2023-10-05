// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"net"
	"os/exec"
	"os/user"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/vm"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"

	subnet "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	"github.com/ava-labs/avalanche-cli/pkg/terraform"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
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
	publicIPMap := map[string]string{}
	gcpProjectName := ""
	gcpCredentialFilepath := ""
	if cloudService == constants.AWSCloudService {
		// Get AWS Credential, region and AMI
		ec2Svc, region, ami, err := getAWSCloudConfig()
		if err != nil {
			return err
		}
		cloudConfig, err = createAWSInstance(ec2Svc, region, ami, usr)
		if err != nil {
			return err
		}
		if !useEIP {
			publicIPMap, err = awsAPI.GetInstancePublicIPs(ec2Svc, cloudConfig.InstanceIDs)
			if err != nil {
				return err
			}
		} else {
			for i, node := range cloudConfig.InstanceIDs {
				publicIPMap[node] = cloudConfig.PublicIPs[i]
			}
		}
	} else {
		cloudConfig, gcpProjectName, gcpCredentialFilepath, err = createGCPInstance(usr)
		if err != nil {
			return err
		}
		for i, node := range cloudConfig.InstanceIDs {
			publicIPMap[node] = cloudConfig.PublicIPs[i]
		}
	}
	if err = createClusterNodeConfig(cloudConfig, clusterName); err != nil {
		return err
	}
	if err = updateClusterConfigGCPKeyFilepath(gcpProjectName, gcpCredentialFilepath); err != nil {
		return err
	}
	err = terraform.RemoveDirectory(app.GetTerraformDir())
	if err != nil {
		return err
	}
	inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	if err = ansible.CreateAnsibleHostInventory(inventoryPath, cloudConfig.CertFilePath, cloudService, publicIPMap); err != nil {
		return err
	}
	time.Sleep(30 * time.Second)

	avalancheGoVersion, err := getAvalancheGoVersion()
	if err != nil {
		return err
	}
	ux.Logger.PrintToUser("Installing AvalancheGo and Avalanche-CLI and starting bootstrap process on the newly created Avalanche node(s) ...")
	if err = runAnsible(inventoryPath, avalancheGoVersion, clusterName); err != nil {
		return err
	}
	if err = setupBuildEnv(clusterName); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Copying staker.crt and staker.key to local machine...")
	for _, instanceID := range cloudConfig.InstanceIDs {
		nodeInstanceDirPath := app.GetNodeInstanceDirPath(instanceID)
		// ansible host alias's name is formatted as ansiblePrefix_{instanceID}
		nodeInstanceAnsibleAlias := fmt.Sprintf("%s_%s", constants.AWSNodeAnsiblePrefix, instanceID)
		if cloudService == constants.GCPCloudService {
			nodeInstanceAnsibleAlias = fmt.Sprintf("%s_%s", constants.GCPNodeAnsiblePrefix, instanceID)
		}
		if err = ansible.RunAnsiblePlaybookCopyStakingFiles(app.GetAnsibleDir(), nodeInstanceAnsibleAlias, nodeInstanceDirPath, inventoryPath); err != nil {
			return err
		}
	}
	PrintResults(cloudConfig, publicIPMap, cloudService)
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
	ux.Logger.PrintToUser("Installing Custom VM build environment on the cloud server(s) ...")
	inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	if err := ansible.RunAnsiblePlaybookSetupBuildEnv(app.GetAnsibleDir(), inventoryPath, "all"); err != nil {
		return err
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

func PrintResults(cloudConfig CloudConfig, publicIPMap map[string]string, cloudService string) {
	ux.Logger.PrintToUser("======================================")
	ux.Logger.PrintToUser("AVALANCHE NODE(S) SUCCESSFULLY SET UP!")
	ux.Logger.PrintToUser("======================================")
	ux.Logger.PrintToUser("Please wait until the node(s) are successfully bootstrapped to run further commands on the node(s)")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Here are the details of the set up node(s): ")
	for i, instanceID := range cloudConfig.InstanceIDs {
		publicIP := ""
		publicIP = publicIPMap[instanceID]
		ux.Logger.PrintToUser("======================================")
		ansibleHostID := fmt.Sprintf("%s_%s", constants.AWSNodeAnsiblePrefix, cloudConfig.InstanceIDs[i])
		if cloudService == constants.GCPCloudService {
			ansibleHostID = fmt.Sprintf("%s_%s", constants.GCPNodeAnsiblePrefix, cloudConfig.InstanceIDs[i])
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Node %s details: ", ansibleHostID))
		ux.Logger.PrintToUser(fmt.Sprintf("Cloud Instance ID: %s", instanceID))
		ux.Logger.PrintToUser(fmt.Sprintf("Public IP: %s", publicIP))
		ux.Logger.PrintToUser(fmt.Sprintf("Cloud Region: %s", cloudConfig.Region))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(fmt.Sprintf("staker.crt and staker.key are stored at %s. If anything happens to your node or the machine node runs on, these files can be used to fully recreate your node.", app.GetNodeInstanceDirPath(instanceID)))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("To ssh to node, run: ")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(fmt.Sprintf("ssh -o IdentitiesOnly=yes -o StrictHostKeyChecking=no ubuntu@%s -i %s", publicIP, cloudConfig.CertFilePath))
		ux.Logger.PrintToUser(utils.GetSSHConnectionString(publicIP, cloudConfig.CertFilePath))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("======================================")
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Don't delete or replace your ssh private key file at %s as you won't be able to access your cloud server without it", cloudConfig.CertFilePath))
	ux.Logger.PrintToUser("")
}
