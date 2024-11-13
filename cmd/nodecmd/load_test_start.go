// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/docker"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/node"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

var (
	loadTestRepoURL    string
	loadTestBuildCmd   string
	loadTestCmd        string
	loadTestRepoCommit string
	repoDirName        string
	loadTestHostRegion string
	loadTestBranch     string
)

type clusterInfo struct {
	API       []nodeInfo `yaml:"API,omitempty"`
	Validator []nodeInfo `yaml:"VALIDATOR,omitempty"`
	LoadTest  nodeInfo   `yaml:"LOADTEST,omitempty"`
	Monitor   nodeInfo   `yaml:"MONITOR,omitempty"`
	ChainID   string     `yaml:"CHAIN_ID,omitempty"`
	SubnetID  string     `yaml:"SUBNET_ID,omitempty"`
}
type nodeInfo struct {
	CloudID string `yaml:"CLOUD_ID,omitempty"`
	NodeID  string `yaml:"NODE_ID,omitempty"`
	IP      string `yaml:"IP,omitempty"`
	Region  string `yaml:"REGION,omitempty"`
}

func newLoadTestStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [loadtestName] [clusterName] [blockchainName]",
		Short: "(ALPHA Warning) Start load test for existing devnet cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node loadtest command starts load testing for an existing devnet cluster. If the cluster does 
not have an existing load test host, the command creates a separate cloud server and builds the load 
test binary based on the provided load test Git Repo URL and load test binary build command. 

The command will then run the load test binary based on the provided load test run command.`,

		Args: cobrautils.ExactArgs(3),
		RunE: startLoadTest,
	}
	cmd.Flags().BoolVar(&useAWS, "aws", false, "create loadtest node in AWS cloud")
	cmd.Flags().BoolVar(&useGCP, "gcp", false, "create loadtest in GCP cloud")
	cmd.Flags().StringVar(&nodeType, "node-type", "", "cloud instance type for loadtest script")
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to create cloud resources")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	cmd.Flags().BoolVar(&useSSHAgent, "use-ssh-agent", false, "use ssh agent(ex: Yubikey) for ssh auth")
	cmd.Flags().StringVar(&sshIdentity, "ssh-agent-identity", "", "use given ssh identity(only for ssh agent). If not set, default will be used")
	cmd.Flags().StringVar(&loadTestRepoURL, "load-test-repo", "", "load test repo url to use")
	cmd.Flags().StringVar(&loadTestBuildCmd, "load-test-build-cmd", "", "command to build load test binary")
	cmd.Flags().StringVar(&loadTestCmd, "load-test-cmd", "", "command to run load test")
	cmd.Flags().StringVar(&loadTestHostRegion, "region", "", "create load test node in a given region")
	cmd.Flags().StringVar(&loadTestBranch, "load-test-branch", "", "load test branch or commit")
	return cmd
}

func preLoadTestChecks(clusterName string) error {
	if err := node.CheckCluster(app, clusterName); err != nil {
		return err
	}
	if useAWS && useGCP {
		return fmt.Errorf("could not use both AWS and GCP cloud options")
	}
	if !useAWS && awsProfile != constants.AWSDefaultCredential {
		return fmt.Errorf("could not use AWS profile for non AWS cloud option")
	}
	if sshIdentity != "" && !useSSHAgent {
		return fmt.Errorf("could not use ssh identity without using ssh agent")
	}
	if useSSHAgent && !utils.IsSSHAgentAvailable() {
		return fmt.Errorf("ssh agent is not available")
	}
	clusterNodes, err := node.GetClusterNodes(app, clusterName)
	if err != nil {
		return err
	}
	if len(clusterNodes) == 0 {
		return fmt.Errorf("no nodes found for loadtesting in the cluster %s", clusterName)
	}
	return nil
}

func startLoadTest(_ *cobra.Command, args []string) error {
	loadTestName := args[0]
	clusterName := args[1]
	blockchainName = args[2]
	if !app.SidecarExists(blockchainName) {
		return fmt.Errorf("subnet %s doesn't exist, please create it first", blockchainName)
	}
	if err := preLoadTestChecks(clusterName); err != nil {
		return err
	}
	loadTestExists, err := checkLoadTestExists(clusterName, loadTestName)
	if err != nil {
		return err
	}
	if loadTestExists {
		return fmt.Errorf("load test %s already exists, please destroy it first", loadTestName)
	}
	var loadTestNodeConfig models.RegionConfig
	var loadTestCloudConfig models.CloudConfig
	// set ssh-Key
	if useSSHAgent && sshIdentity == "" {
		sshIdentity, err = setSSHIdentity()
		if err != nil {
			return err
		}
	}
	clusterNodes, err := node.GetClusterNodes(app, clusterName)
	if err != nil {
		return err
	}
	existingSeparateInstance, err = getExistingLoadTestInstance(clusterName, loadTestName)
	if err != nil {
		return err
	}
	cloudService := ""
	if existingSeparateInstance != "" {
		ux.Logger.PrintToUser("Will be using cloud instance %s to run load test...", existingSeparateInstance)
		separateNodeConfig, err := app.LoadClusterNodeConfig(existingSeparateInstance)
		if err != nil {
			return err
		}
		cloudService = separateNodeConfig.CloudService
	} else {
		ux.Logger.PrintToUser("Creating a separate instance to run load test...")
		cloudService, err = setCloudService()
		if err != nil {
			return err
		}
		nodeType, err = setCloudInstanceType(cloudService)
		if err != nil {
			return err
		}
	}
	separateHostRegion := ""
	cloudSecurityGroupList, err := getCloudSecurityGroupList(clusterNodes)
	if err != nil {
		return err
	}
	// we only support endpoint in the same cluster
	filteredSGList := utils.Filter(cloudSecurityGroupList, func(sg regionSecurityGroup) bool { return sg.cloud == cloudService })
	if len(filteredSGList) == 0 {
		return fmt.Errorf("no endpoint found in the  %s", cloudService)
	}
	sgRegions := []string{}
	for index := range filteredSGList {
		sgRegions = append(sgRegions, filteredSGList[index].region)
	}
	// if no loadtest region is provided, use some region of the cluster
	if loadTestHostRegion == "" {
		loadTestHostRegion = sgRegions[0]
	}
	if !slices.Contains(sgRegions, loadTestHostRegion) {
		sgRegions = append(sgRegions, loadTestHostRegion)
	}
	switch cloudService {
	case constants.AWSCloudService:
		var ec2SvcMap map[string]*awsAPI.AwsCloud
		var ami map[string]string
		loadTestEc2SvcMap := make(map[string]*awsAPI.AwsCloud)
		if existingSeparateInstance == "" {
			ec2SvcMap, ami, _, err = getAWSCloudConfig(awsProfile, true, sgRegions, nodeType)
			if err != nil {
				return err
			}
			separateHostRegion = loadTestHostRegion
			loadTestEc2SvcMap[separateHostRegion] = ec2SvcMap[separateHostRegion]
			loadTestCloudConfig, err = createAWSInstances(loadTestEc2SvcMap, nodeType, map[string]NumNodes{separateHostRegion: {1, 0}}, []string{separateHostRegion}, ami, true, false)
			if err != nil {
				return err
			}
			loadTestNodeConfig = loadTestCloudConfig[separateHostRegion]
		} else {
			loadTestNodeConfig, separateHostRegion, err = getNodeCloudConfig(existingSeparateInstance)
			if err != nil {
				return err
			}
			loadTestEc2SvcMap, err = getAWSMonitoringEC2Svc(awsProfile, separateHostRegion)
			if err != nil {
				return err
			}
		}
		if !useStaticIP {
			// get loadtest public
			loadTestPublicIPMap, err := loadTestEc2SvcMap[separateHostRegion].GetInstancePublicIPs(loadTestNodeConfig.InstanceIDs)
			if err != nil {
				return err
			}
			loadTestNodeConfig.PublicIPs = []string{loadTestPublicIPMap[loadTestNodeConfig.InstanceIDs[0]]}
		}
		if existingSeparateInstance == "" {
			for _, sg := range filteredSGList {
				if err = grantAccessToPublicIPViaSecurityGroup(ec2SvcMap[sg.region], loadTestNodeConfig.PublicIPs[0], sg.securityGroup, sg.region); err != nil {
					return err
				}
			}
		}
	case constants.GCPCloudService:
		var gcpClient *gcpAPI.GcpCloud
		var gcpRegions map[string]NumNodes
		var imageID string
		var projectName string
		if existingSeparateInstance == "" {
			// Get GCP Credential, zone, Image ID, service account key file path, and GCP project name
			gcpClient, gcpRegions, imageID, _, projectName, err = getGCPConfig(true)
			if err != nil {
				return err
			}
			regions := maps.Keys(gcpRegions)
			separateHostRegion = regions[0]
			loadTestCloudConfig, err = createGCPInstance(gcpClient, nodeType, map[string]NumNodes{separateHostRegion: {1, 0}}, imageID, clusterName, true)
			if err != nil {
				return err
			}
			loadTestNodeConfig = loadTestCloudConfig[separateHostRegion]
		} else {
			_, projectName, _, err = getGCPCloudCredentials()
			if err != nil {
				return err
			}
			loadTestNodeConfig, separateHostRegion, err = getNodeCloudConfig(existingSeparateInstance)
			if err != nil {
				return err
			}
		}
		if !useStaticIP {
			loadTestPublicIPMap, err := gcpClient.GetInstancePublicIPs(separateHostRegion, loadTestNodeConfig.InstanceIDs)
			if err != nil {
				return err
			}
			loadTestNodeConfig.PublicIPs = []string{loadTestPublicIPMap[loadTestNodeConfig.InstanceIDs[0]]}
		}
		if existingSeparateInstance == "" {
			if err = grantAccessToPublicIPViaFirewall(gcpClient, projectName, loadTestNodeConfig.PublicIPs[0], "loadtest"); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("cloud service %s is not supported", cloudService)
	}
	if existingSeparateInstance == "" {
		if err := saveExternalHostConfig(loadTestNodeConfig, separateHostRegion, cloudService, clusterName, constants.LoadTestRole, loadTestName); err != nil {
			return err
		}
	}
	// separateHosts contains all load test hosts defined in load test inventory dir
	var separateHosts []*models.Host
	// separateHosts contains only current load test host defined in the command
	var currentLoadTestHost []*models.Host
	separateHostInventoryPath := app.GetLoadTestInventoryDir(clusterName)
	if existingSeparateInstance == "" {
		if err = ansible.CreateAnsibleHostInventory(separateHostInventoryPath, loadTestNodeConfig.CertFilePath, cloudService, map[string]string{loadTestNodeConfig.InstanceIDs[0]: loadTestNodeConfig.PublicIPs[0]}, nil); err != nil {
			return err
		}
	}
	separateHosts, err = ansible.GetInventoryFromAnsibleInventoryFile(separateHostInventoryPath)
	if err != nil {
		return err
	}

	for _, host := range separateHosts {
		if host.GetCloudID() == loadTestNodeConfig.InstanceIDs[0] {
			currentLoadTestHost = append(currentLoadTestHost, host)
		}
	}
	if err := GetLoadTestScript(app); err != nil {
		return err
	}

	// waiting for all nodes to become accessible
	if existingSeparateInstance == "" {
		failedHosts := waitForHosts(currentLoadTestHost)
		if failedHosts.Len() > 0 {
			for _, result := range failedHosts.GetResults() {
				ux.Logger.PrintToUser("Instance %s failed to setup with error %s. Please check instance logs for more information", result.NodeID, result.Err)
			}
			return fmt.Errorf("failed to setup node(s) %s", failedHosts.GetNodeList())
		}
		ux.Logger.PrintToUser("Separate instance %s provisioned successfully", currentLoadTestHost[0].NodeID)
	}
	ux.Logger.PrintToUser("Setting up load test environment")
	if err := ssh.RunSSHBuildLoadTestDependencies(currentLoadTestHost[0]); err != nil {
		return err
	}
	monitoringInventoryPath := app.GetMonitoringInventoryDir(clusterName)
	monitoringHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryPath)
	if err != nil {
		return err
	}
	if len(monitoringHosts) > 0 {
		if err := ssh.RunSSHSetupPromtailConfig(currentLoadTestHost[0], monitoringHosts[0].IP, constants.AvalanchegoLokiPort, currentLoadTestHost[0].GetCloudID(), "NodeID-Loadtest", ""); err != nil {
			return err
		}
		if err := ssh.RunSSHSetupDockerService(currentLoadTestHost[0]); err != nil {
			return err
		}
		if err := docker.ComposeSSHSetupLoadTest(currentLoadTestHost[0]); err != nil {
			return err
		}
		avalancheGoPorts, machinePorts, ltPorts, err := getPrometheusTargets(clusterName)
		if err != nil {
			return err
		}
		if err := ssh.RunSSHSetupPrometheusConfig(monitoringHosts[0], avalancheGoPorts, machinePorts, ltPorts); err != nil {
			return err
		}
		if err := docker.RestartDockerComposeService(monitoringHosts[0], utils.GetRemoteComposeFile(), "prometheus", constants.SSHLongRunningScriptTimeout); err != nil {
			return err
		}
	}

	subnetID, chainID, err := getDeployedSubnetInfo(clusterName, blockchainName)
	if err != nil {
		return err
	}

	if err := createClusterYAMLFile(clusterName, subnetID, chainID, currentLoadTestHost[0]); err != nil {
		return err
	}

	if err := ssh.RunSSHCopyYAMLFile(currentLoadTestHost[0], app.GetClusterYAMLFilePath(clusterName)); err != nil {
		return err
	}
	checkoutCommit := false
	if loadTestRepoCommit != "" {
		checkoutCommit = true
	}

	ux.Logger.GreenCheckmarkToUser("Load test environment is ready!")
	ux.Logger.PrintToUser("%s Building load test code", logging.Green.Wrap(">"))
	if err := ssh.RunSSHBuildLoadTestCode(currentLoadTestHost[0], loadTestRepoURL, loadTestBuildCmd, loadTestRepoCommit, repoDirName, loadTestBranch, checkoutCommit); err != nil {
		return err
	}

	ux.Logger.PrintToUser("%s Running load test", logging.Green.Wrap(">"))
	if err := ssh.RunSSHRunLoadTest(currentLoadTestHost[0], loadTestCmd, loadTestName); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Load test successfully run!")
	return nil
}

func getDeployedSubnetInfo(clusterName string, blockchainName string) (string, string, error) {
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return "", "", err
	}
	network, err := app.GetClusterNetwork(clusterName)
	if err != nil {
		return "", "", err
	}

	if sc.Networks != nil {
		model, ok := sc.Networks[network.Name()]
		if ok {
			if model.SubnetID != ids.Empty && model.BlockchainID != ids.Empty {
				return model.SubnetID.String(), model.BlockchainID.String(), nil
			}
		}
	}
	return "", "", fmt.Errorf("unable to find deployed Cluster info, please call avalanche subnet deploy <blockchainName> --cluster <clusterName> first")
}

func createClusterYAMLFile(clusterName, subnetID, chainID string, separateHost *models.Host) error {
	clusterYAMLFilePath := filepath.Join(app.GetAnsibleInventoryDirPath(clusterName), constants.ClusterYAMLFileName)
	if utils.FileExists(clusterYAMLFilePath) {
		if err := os.Remove(clusterYAMLFilePath); err != nil {
			return err
		}
	}
	yamlFile, err := os.OpenFile(clusterYAMLFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, constants.WriteReadReadPerms)
	if err != nil {
		return err
	}
	defer yamlFile.Close()

	enc := yaml.NewEncoder(yamlFile)

	clusterConf, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	if err := node.CheckCluster(app, clusterName); err != nil {
		return err
	}
	var apiNodes []nodeInfo
	var validatorNodes []nodeInfo
	var monitorNode nodeInfo
	for _, cloudID := range clusterConf.GetCloudIDs() {
		nodeConfig, err := app.LoadClusterNodeConfig(cloudID)
		if err != nil {
			return err
		}
		nodeIDStr := ""
		if clusterConf.IsAvalancheGoHost(cloudID) {
			nodeID, err := getNodeID(app.GetNodeInstanceDirPath(cloudID))
			if err != nil {
				return err
			}
			nodeIDStr = nodeID.String()
		}
		roles := clusterConf.GetHostRoles(nodeConfig)
		if len(roles) == 0 {
			return fmt.Errorf("incorrect node config file at %s", app.GetNodeConfigPath(cloudID))
		}
		switch roles[0] {
		case constants.ValidatorRole:
			validatorNode := nodeInfo{
				CloudID: cloudID,
				NodeID:  nodeIDStr,
				IP:      nodeConfig.ElasticIP,
				Region:  nodeConfig.Region,
			}
			validatorNodes = append(validatorNodes, validatorNode)
		case constants.APIRole:
			apiNode := nodeInfo{
				CloudID: cloudID,
				IP:      nodeConfig.ElasticIP,
				Region:  nodeConfig.Region,
			}
			apiNodes = append(apiNodes, apiNode)
		case constants.MonitorRole:
			monitorNode = nodeInfo{
				CloudID: cloudID,
				IP:      nodeConfig.ElasticIP,
				Region:  nodeConfig.Region,
			}
		default:
		}
	}
	var separateHostInfo nodeInfo
	if separateHost != nil {
		_, separateHostRegion, err := getNodeCloudConfig(separateHost.GetCloudID())
		if err != nil {
			return err
		}
		separateHostInfo = nodeInfo{
			IP:      separateHost.IP,
			CloudID: separateHost.GetCloudID(),
			Region:  separateHostRegion,
		}
	}
	clusterInfoYAML := clusterInfo{
		Validator: validatorNodes,
		API:       apiNodes,
		LoadTest:  separateHostInfo,
		Monitor:   monitorNode,
		SubnetID:  subnetID,
		ChainID:   chainID,
	}
	return enc.Encode(clusterInfoYAML)
}

func GetLoadTestScript(app *application.Avalanche) error {
	var err error
	if loadTestRepoURL != "" {
		ux.Logger.PrintToUser("Checking source code repository URL %s", loadTestRepoURL)
		if err := prompts.ValidateURL(loadTestRepoURL); err != nil {
			ux.Logger.PrintToUser("Invalid repository url %s: %s", loadTestRepoURL, err)
			loadTestRepoURL = ""
		}
	}
	if loadTestRepoURL == "" {
		loadTestRepoURL, err = app.Prompt.CaptureURL("Source code repository URL", true)
		if err != nil {
			return err
		}
	}
	loadTestRepoCommit = utils.GetGitCommit(loadTestRepoURL)
	if loadTestRepoCommit != "" {
		loadTestBranch = loadTestRepoCommit
	}
	if loadTestBranch == "" {
		loadTestBranch, err = app.Prompt.CaptureString("Which branch / commit of the load test repository do you want to use?")
		if err != nil {
			return err
		}
	}
	loadTestRepoURL, repoDirName = utils.GetRepoFromCommitURL(loadTestRepoURL)
	if loadTestBuildCmd == "" {
		loadTestBuildCmd, err = app.Prompt.CaptureString("What is the build command?")
		if err != nil {
			return err
		}
	}
	if loadTestCmd == "" {
		loadTestCmd, err = app.Prompt.CaptureString("What is the load test command?")
		if err != nil {
			return err
		}
	}
	return nil
}

func getExistingLoadTestInstance(clusterName, loadTestName string) (string, error) {
	clustersConfig, err := app.GetClustersConfig()
	if err != nil {
		return "", err
	}
	if _, ok := clustersConfig.Clusters[clusterName]; ok {
		if _, loadTestExists := clustersConfig.Clusters[clusterName].LoadTestInstance[loadTestName]; loadTestExists {
			return clustersConfig.Clusters[clusterName].LoadTestInstance[loadTestName], nil
		}
	}
	return "", nil
}
