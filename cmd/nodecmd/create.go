// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/terraform"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"

	subnet "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"

	"golang.org/x/sync/errgroup"
)

const (
	avalancheGoReferenceChoiceLatest = "latest"
	avalancheGoReferenceChoiceSubnet = "subnet"
	avalancheGoReferenceChoiceCustom = "custom"
)

var (
	useAWS                          bool
	useGCP                          bool
	cmdLineRegion                   string
	authorizeAccess                 bool
	numNodes                        int
	useLatestAvalanchegoVersion     bool
	useAvalanchegoVersionFromSubnet string
	cmdLineGCPCredentialsPath       string
	cmdLineGCPProjectName           string
	cmdLineAlternativeKeyPairName   string
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
		RunE:         createNodes,
	}
	cmd.Flags().BoolVar(&useStaticIP, "use-static-ip", true, "attach static Public IP on cloud servers")
	cmd.Flags().BoolVar(&useAWS, "aws", false, "create node/s in AWS cloud")
	cmd.Flags().BoolVar(&useGCP, "gcp", false, "create node/s in GCP cloud")
	cmd.Flags().StringVar(&cmdLineRegion, "region", "", "create node/s in given region")
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to create cloud resources")
	cmd.Flags().IntVar(&numNodes, "num-nodes", 0, "number of nodes to create")
	cmd.Flags().BoolVar(&useLatestAvalanchegoVersion, "latest-avalanchego-version", false, "install latest avalanchego version on node/s")
	cmd.Flags().StringVar(&useAvalanchegoVersionFromSubnet, "avalanchego-version-from-subnet", "", "install latest avalanchego version, that is compatible with the given subnet, on node/s")
	cmd.Flags().StringVar(&cmdLineGCPCredentialsPath, "gcp-credentials", "", "use given GCP credentials")
	cmd.Flags().StringVar(&cmdLineGCPProjectName, "gcp-project", "", "use given GCP project")
	cmd.Flags().StringVar(&cmdLineAlternativeKeyPairName, "alternative-key-pair-name", "", "key pair name to use if default one generates conflicts")

	return cmd
}

func createNodes(_ *cobra.Command, args []string) error {
	if useLatestAvalanchegoVersion && useAvalanchegoVersionFromSubnet != "" {
		return fmt.Errorf("could not use both latest avalanchego version and avalanchego version based on given subnet")
	}
	if useAWS && useGCP {
		return fmt.Errorf("could not use both AWS and GCP cloud options")
	}
	clusterName := args[0]
	cloudService, err := setCloudService()
	if err != nil {
		return err
	}
	if cloudService != constants.GCPCloudService && cmdLineGCPCredentialsPath != "" {
		return fmt.Errorf("set to use GCP credentials but cloud option is not GCP")
	}
	if cloudService != constants.GCPCloudService && cmdLineGCPProjectName != "" {
		return fmt.Errorf("set to use GCP project but cloud option is not GCP")
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
		ec2Svc, region, ami, err := getAWSCloudConfig(cmdLineRegion, authorizeAccess)
		if err != nil {
			return err
		}
		cloudConfig, err = createAWSInstances(ec2Svc, numNodes, region, ami, usr)
		if err != nil {
			return err
		}
		if !useStaticIP {
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
		// Get GCP Credential, zone, Image ID, service account key file path, and GCP project name
		gcpClient, zone, imageID, credentialFilepath, projectName, err := getGCPConfig(cmdLineRegion)
		if err != nil {
			return err
		}
		cloudConfig, err = createGCPInstance(usr, gcpClient, numNodes, zone, imageID, credentialFilepath, projectName, clusterName)
		if err != nil {
			return err
		}
		if !useStaticIP {
			publicIPMap, err = gcpAPI.GetInstancePublicIPs(gcpClient, projectName, zone, cloudConfig.InstanceIDs)
			if err != nil {
				return err
			}
		} else {
			for i, node := range cloudConfig.InstanceIDs {
				publicIPMap[node] = cloudConfig.PublicIPs[i]
			}
		}
		gcpProjectName = projectName
		gcpCredentialFilepath = credentialFilepath
	}
	if err = createClusterNodeConfig(cloudConfig, clusterName, cloudService); err != nil {
		return err
	}
	if cloudService == constants.GCPCloudService {
		if err = updateClusterConfigGCPKeyFilepath(gcpProjectName, gcpCredentialFilepath); err != nil {
			return err
		}
	}
	err = terraform.RemoveDirectory(app.GetTerraformDir())
	if err != nil {
		return err
	}

	time.Sleep(30 * time.Second)

	avalancheGoVersion, err := getAvalancheGoVersion()
	if err != nil {
		return err
	}
	inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	if err = ansible.CreateAnsibleHostInventory(inventoryPath, cloudConfig.CertFilePath, cloudService, publicIPMap); err != nil {
		return err
	}

	ux.Logger.PrintToUser("Installing AvalancheGo and Avalanche-CLI and starting bootstrap process on the newly created Avalanche node(s) ...")
	ansibleHostIDs, err := utils.MapWithError(cloudConfig.InstanceIDs, func(s string) (string, error) { return models.HostCloudIDToAnsibleID(cloudService, s) })
	if err != nil {
		return err
	}
	createdAnsibleHostIDs := strings.Join(ansibleHostIDs, ",")
	if err = runAnsible(inventoryPath, avalancheGoVersion, clusterName, createdAnsibleHostIDs); err != nil {
		return err
	}
	if err = setupBuildEnv(inventoryPath, createdAnsibleHostIDs); err != nil {
		return err
	}
	printResults(cloudConfig, publicIPMap, ansibleHostIDs)
	ux.Logger.PrintToUser("AvalancheGo and Avalanche-CLI installed and node(s) are bootstrapping!")
	return nil
}

// createClusterNodeConfig creates node config and save it in .avalanche-cli/nodes/{instanceID}
// also creates cluster config in .avalanche-cli/nodes storing various key pair and security group info for all clusters
// func createClusterNodeConfig(nodeIDs, publicIPs []string, region, ami, keyPairName, certPath, sg, clusterName string) error {
func createClusterNodeConfig(cloudConfig CloudConfig, clusterName, cloudService string) error {
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
			CloudService:  cloudService,
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

func runAnsible(inventoryPath, avalancheGoVersion, clusterName, ansibleHostIDs string) error {
	if err := setupAnsible(clusterName); err != nil {
		return err
	}
	if err := distributeStakingCertAndKey(strings.Split(ansibleHostIDs, ","), inventoryPath); err != nil {
		return err
	}
	return ansible.RunAnsiblePlaybookSetupNode(app.GetConfigPath(), app.GetAnsibleDir(), inventoryPath, avalancheGoVersion, ansibleHostIDs)
}

func setupBuildEnv(inventoryPath, ansibleHostIDs string) error {
	ux.Logger.PrintToUser("Installing Custom VM build environment on the cloud server(s) ...")
	ansibleTargetHosts := "all"
	if ansibleHostIDs != "" {
		ansibleTargetHosts = ansibleHostIDs
	}
	return ansible.RunAnsiblePlaybookSetupBuildEnv(app.GetAnsibleDir(), inventoryPath, ansibleTargetHosts)
}

func getNodeID(nodeDir string) (ids.NodeID, error) {
	certBytes, err := os.ReadFile(filepath.Join(nodeDir, constants.StakerCertFileName))
	if err != nil {
		return ids.EmptyNodeID, err
	}
	keyBytes, err := os.ReadFile(filepath.Join(nodeDir, constants.StakerKeyFileName))
	if err != nil {
		return ids.EmptyNodeID, err
	}
	nodeID, err := utils.ToNodeID(certBytes, keyBytes)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	return nodeID, nil
}

func generateNodeCertAndKeys(stakerCertFilePath, stakerKeyFilePath, blsKeyFilePath string) (ids.NodeID, error) {
	certBytes, keyBytes, err := staking.NewCertAndKeyBytes()
	if err != nil {
		return ids.EmptyNodeID, err
	}
	nodeID, err := utils.ToNodeID(certBytes, keyBytes)
	if err != nil {
		return ids.EmptyNodeID, err
	}
	if err := os.MkdirAll(filepath.Dir(stakerCertFilePath), constants.DefaultPerms755); err != nil {
		return ids.EmptyNodeID, err
	}
	if err := os.WriteFile(stakerCertFilePath, certBytes, constants.WriteReadUserOnlyPerms); err != nil {
		return ids.EmptyNodeID, err
	}
	if err := os.MkdirAll(filepath.Dir(stakerKeyFilePath), constants.DefaultPerms755); err != nil {
		return ids.EmptyNodeID, err
	}
	if err := os.WriteFile(stakerKeyFilePath, keyBytes, constants.WriteReadUserOnlyPerms); err != nil {
		return ids.EmptyNodeID, err
	}
	blsSignerKeyBytes, err := utils.NewBlsSecretKeyBytes()
	if err != nil {
		return ids.EmptyNodeID, err
	}
	if err := os.MkdirAll(filepath.Dir(blsKeyFilePath), constants.DefaultPerms755); err != nil {
		return ids.EmptyNodeID, err
	}
	if err := os.WriteFile(blsKeyFilePath, blsSignerKeyBytes, constants.WriteReadUserOnlyPerms); err != nil {
		return ids.EmptyNodeID, err
	}
	return nodeID, nil
}

func distributeStakingCertAndKey(ansibleHostIDs []string, inventoryPath string) error {
	ux.Logger.PrintToUser("Generating staking keys in local machine...")
	eg := errgroup.Group{}
	for _, ansibleInstanceID := range ansibleHostIDs {
		_, instanceID, err := models.HostAnsibleIDToCloudID(ansibleInstanceID)
		if err != nil {
			return err
		}
		keyPath := filepath.Join(app.GetNodesDir(), instanceID)
		eg.Go(func() error {
			nodeID, err := generateNodeCertAndKeys(
				filepath.Join(keyPath, constants.StakerCertFileName),
				filepath.Join(keyPath, constants.StakerKeyFileName),
				filepath.Join(keyPath, constants.BLSKeyFileName),
			)
			if err != nil {
				ux.Logger.PrintToUser("Failed to generate staking keys for host %s", instanceID)
				return err
			} else {
				ux.Logger.PrintToUser("Generated staking keys for host %s[%s] ", instanceID, nodeID.String())
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Copying staking keys to remote machine(s)...")
	return ansible.RunAnsiblePlaybookCopyStakingFiles(app.GetAnsibleDir(), strings.Join(ansibleHostIDs, ","), app.GetNodesDir(), inventoryPath)
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
	version := ""
	subnet := ""
	if useLatestAvalanchegoVersion { //nolint: gocritic
		version = "latest"
	} else if useAvalanchegoVersionFromSubnet != "" {
		subnet = useAvalanchegoVersionFromSubnet
	} else {
		choice, subnetChoice, err := promptAvalancheGoReferenceChoice()
		if err != nil {
			return "", err
		}
		switch choice {
		case avalancheGoReferenceChoiceLatest:
			version = "latest"
		case avalancheGoReferenceChoiceCustom:
			customVersion, err := app.Prompt.CaptureString("Which version of AvalancheGo would you like to install? (Use format v1.10.13)")
			if err != nil {
				return "", err
			}
			if !strings.HasPrefix(customVersion, "v") {
				return "", errors.New("invalid avalanche go version")
			}
			version = customVersion
		case avalancheGoReferenceChoiceSubnet:
			subnet = subnetChoice
		}
	}
	if subnet != "" {
		sc, err := app.LoadSidecar(subnet)
		if err != nil {
			return "", err
		}
		version, err = GetLatestAvagoVersionForRPC(sc.RPCVersion)
		if err != nil {
			return "", err
		}
	}
	return version, nil
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
func promptAvalancheGoReferenceChoice() (string, string, error) {
	defaultVersion := "Use latest Avalanche Go Version"
	txt := "What version of Avalanche Go would you like to install in the node?"
	versionOptions := []string{defaultVersion, "Use the deployed Subnet's VM version that the node will be validating", "Custom"}
	versionOption, err := app.Prompt.CaptureList(txt, versionOptions)
	if err != nil {
		return "", "", err
	}

	switch versionOption {
	case defaultVersion:
		return avalancheGoReferenceChoiceLatest, "", nil
	case "Custom":
		return avalancheGoReferenceChoiceCustom, "", nil
	default:
		for {
			subnetName, err := app.Prompt.CaptureString("Which Subnet would you like to use to choose the avalanche go version?")
			if err != nil {
				return "", "", err
			}
			_, err = subnet.ValidateSubnetNameAndGetChains([]string{subnetName})
			if err == nil {
				return avalancheGoReferenceChoiceSubnet, subnetName, nil
			}
			ux.Logger.PrintToUser(fmt.Sprintf("no subnet named %s found", subnetName))
		}
	}
}

func setCloudService() (string, error) {
	if useAWS {
		return constants.AWSCloudService, nil
	}
	if useGCP {
		return constants.GCPCloudService, nil
	}
	txt := "Which cloud service would you like to launch your Avalanche Node(s) in?"
	cloudOptions := []string{constants.AWSCloudService, constants.GCPCloudService}
	chosenCloudService, err := app.Prompt.CaptureList(txt, cloudOptions)
	if err != nil {
		return "", err
	}
	return chosenCloudService, nil
}

func printResults(cloudConfig CloudConfig, publicIPMap map[string]string, ansibleHostIDs []string) {
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
		ux.Logger.PrintToUser(fmt.Sprintf("Node %s details: ", ansibleHostIDs[i]))
		ux.Logger.PrintToUser(fmt.Sprintf("Cloud Instance ID: %s", instanceID))
		ux.Logger.PrintToUser(fmt.Sprintf("Public IP: %s", publicIP))
		ux.Logger.PrintToUser(fmt.Sprintf("Cloud Region: %s", cloudConfig.Region))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(fmt.Sprintf("staker.crt and staker.key are stored at %s. If anything happens to your node or the machine node runs on, these files can be used to fully recreate your node.", app.GetNodeInstanceDirPath(instanceID)))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("To ssh to node, run: ")
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser(utils.GetSSHConnectionString(publicIP, cloudConfig.CertFilePath))
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("======================================")
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Don't delete or replace your ssh private key file at %s as you won't be able to access your cloud server without it", cloudConfig.CertFilePath))
	ux.Logger.PrintToUser("")
}
