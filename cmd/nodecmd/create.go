// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

const (
	avalancheGoReferenceChoiceLatest = "latest"
	avalancheGoReferenceChoiceSubnet = "subnet"
	avalancheGoReferenceChoiceCustom = "custom"
)

var (
	createOnFuji                    bool
	createDevnet                    bool
	createOnMainnet                 bool
	useAWS                          bool
	useGCP                          bool
	cmdLineRegion                   []string
	authorizeAccess                 bool
	numNodes                        []int
	nodeType                        string
	existingMonitoringInstance      string
	useLatestAvalanchegoVersion     bool
	useCustomAvalanchegoVersion     string
	useAvalanchegoVersionFromSubnet string
	cmdLineGCPCredentialsPath       string
	cmdLineGCPProjectName           string
	cmdLineAlternativeKeyPairName   string
	useSSHAgent                     bool
	sshIdentity                     string
	setUpMonitoring                 bool
	skipMonitoring                  bool
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
		RunE:         createNodes,
	}
	cmd.Flags().BoolVar(&useStaticIP, "use-static-ip", true, "attach static Public IP on cloud servers")
	cmd.Flags().BoolVar(&useAWS, "aws", false, "create node/s in AWS cloud")
	cmd.Flags().BoolVar(&useGCP, "gcp", false, "create node/s in GCP cloud")
	cmd.Flags().StringSliceVar(&cmdLineRegion, "region", []string{}, "create node(s) in given region(s). Use comma to separate multiple regions")
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to create cloud resources")
	cmd.Flags().IntSliceVar(&numNodes, "num-nodes", []int{}, "number of nodes to create per region(s). Use comma to separate multiple numbers for each region in the same order as --region flag")
	cmd.Flags().StringVar(&nodeType, "node-type", "", "cloud instance type. Use 'default' to use recommended default instance type")
	cmd.Flags().BoolVar(&useLatestAvalanchegoVersion, "latest-avalanchego-version", false, "install latest avalanchego version on node/s")
	cmd.Flags().StringVar(&useCustomAvalanchegoVersion, "custom-avalanchego-version", "", "install given avalanchego version on node/s")
	cmd.Flags().StringVar(&useAvalanchegoVersionFromSubnet, "avalanchego-version-from-subnet", "", "install latest avalanchego version, that is compatible with the given subnet, on node/s")
	cmd.Flags().StringVar(&cmdLineGCPCredentialsPath, "gcp-credentials", "", "use given GCP credentials")
	cmd.Flags().StringVar(&cmdLineGCPProjectName, "gcp-project", "", "use given GCP project")
	cmd.Flags().StringVar(&cmdLineAlternativeKeyPairName, "alternative-key-pair-name", "", "key pair name to use if default one generates conflicts")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	cmd.Flags().BoolVar(&createOnFuji, "fuji", false, "create node/s in Fuji Network")
	cmd.Flags().BoolVar(&createDevnet, "devnet", false, "create node/s into a new Devnet")
	cmd.Flags().BoolVar(&useSSHAgent, "use-ssh-agent", false, "use ssh agent(ex: Yubikey) for ssh auth")
	cmd.Flags().StringVar(&sshIdentity, "ssh-agent-identity", "", "use given ssh identity(only for ssh agent). If not set, default will be used.")
	cmd.Flags().BoolVar(&sameMonitoringInstance, "same-monitoring-instance", false, "host monitoring for a cloud servers on the same instance")
	cmd.Flags().BoolVar(&separateMonitoringInstance, "separate-monitoring-instance", false, "host monitoring for all cloud servers on a separate instance")
	cmd.Flags().BoolVar(&skipMonitoring, "skip-monitoring", false, "don't set up monitoring in created nodes")
	return cmd
}

func preCreateChecks() error {
	if !flags.EnsureMutuallyExclusive([]bool{useLatestAvalanchegoVersion, useAvalanchegoVersionFromSubnet != "", useCustomAvalanchegoVersion != ""}) {
		return fmt.Errorf("latest avalanchego version, custom avalanchego version and avalanchego version based on given subnet, are mutually exclusive options")
	}
	if useAWS && useGCP {
		return fmt.Errorf("could not use both AWS and GCP cloud options")
	}
	if !useAWS && awsProfile != constants.AWSDefaultCredential {
		return fmt.Errorf("could not use AWS profile for non AWS cloud option")
	}
	if len(utils.Unique(cmdLineRegion)) != len(numNodes) {
		return fmt.Errorf("number of regions and number of nodes must be equal. Please make sure list of regions is unique")
	}
	if len(numNodes) > 0 {
		for _, num := range numNodes {
			if num <= 0 {
				return fmt.Errorf("number of nodes per region must be greater than 0")
			}
		}
	}
	if sshIdentity != "" && !useSSHAgent {
		return fmt.Errorf("could not use ssh identity without using ssh agent")
	}
	if useSSHAgent && !utils.IsSSHAgentAvailable() {
		return fmt.Errorf("ssh agent is not available")
	}
	return nil
}

func createNodes(_ *cobra.Command, args []string) error {
	if err := preCreateChecks(); err != nil {
		return err
	}
	clusterName := args[0]

	network, err := subnetcmd.GetNetworkFromCmdLineFlags(
		false,
		createDevnet,
		createOnFuji,
		createOnMainnet,
		"",
		false,
		[]models.NetworkKind{models.Fuji, models.Devnet},
	)
	if err != nil {
		return err
	}

	cloudService, err := setCloudService()
	if err != nil {
		return err
	}
	nodeType, err = setCloudInstanceType(cloudService)
	if err != nil {
		return err
	}

	if cloudService != constants.GCPCloudService && cmdLineGCPCredentialsPath != "" {
		return fmt.Errorf("set to use GCP credentials but cloud option is not GCP")
	}
	if cloudService != constants.GCPCloudService && cmdLineGCPProjectName != "" {
		return fmt.Errorf("set to use GCP project but cloud option is not GCP")
	}
	usr, err := user.Current()
	if err != nil {
		return err
	}
	cloudConfigMap := models.CloudConfig{}
	publicIPMap := map[string]string{}
	gcpProjectName := ""
	gcpCredentialFilepath := ""
	// set ssh-Key
	if useSSHAgent && sshIdentity == "" {
		sshIdentity, err = setSSHIdentity()
		if err != nil {
			return err
		}
	}
	monitoringHostRegion := ""
	monitoringNodeConfig := models.RegionConfig{}
	existingMonitoringInstance, err = getExistingMonitoringInstance(clusterName)
	if err != nil {
		return err
	}
	if cloudService == constants.AWSCloudService { // Get AWS Credential, region and AMI
		if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.AWSCloudService) != nil) {
			return fmt.Errorf("cloud access is required")
		}
		ec2SvcMap, ami, numNodesMap, err := getAWSCloudConfig(awsProfile)
		regions := maps.Keys(ec2SvcMap)
		if err != nil {
			return err
		}
		if existingMonitoringInstance == "" {
			monitoringHostRegion = regions[0]
		}
		if !skipMonitoring {
			setUpMonitoring, separateMonitoringInstance, err = promptSetUpMonitoring()
			if err != nil {
				return err
			}
		}
		cloudConfigMap, err = createAWSInstances(ec2SvcMap, nodeType, numNodesMap, regions, ami, usr, false)
		if err != nil {
			return err
		}
		monitoringEc2SvcMap := make(map[string]*awsAPI.AwsCloud)
		if separateMonitoringInstance && existingMonitoringInstance == "" {
			monitoringEc2SvcMap[monitoringHostRegion] = ec2SvcMap[monitoringHostRegion]
			monitoringCloudConfig, err := createAWSInstances(monitoringEc2SvcMap, nodeType, map[string]int{monitoringHostRegion: 1}, []string{monitoringHostRegion}, ami, usr, true)
			if err != nil {
				return err
			}
			monitoringNodeConfig = monitoringCloudConfig[regions[0]]
		}
		if existingMonitoringInstance != "" {
			separateMonitoringInstance = true
			monitoringNodeConfig, monitoringHostRegion, err = getNodeCloudConfig(existingMonitoringInstance)
			if err != nil {
				return err
			}
			monitoringEc2SvcMap, err = getAWSMonitoringEC2Svc(awsProfile, monitoringHostRegion)
			if err != nil {
				return err
			}
		}
		if !useStaticIP && separateMonitoringInstance {
			monitoringPublicIPMap, err := monitoringEc2SvcMap[monitoringHostRegion].GetInstancePublicIPs(monitoringNodeConfig.InstanceIDs)
			if err != nil {
				return err
			}
			monitoringNodeConfig.PublicIPs = []string{monitoringPublicIPMap[monitoringNodeConfig.InstanceIDs[0]]}
		}
		for _, region := range regions {
			if !useStaticIP {
				tmpIPMap, err := ec2SvcMap[region].GetInstancePublicIPs(cloudConfigMap[region].InstanceIDs)
				if err != nil {
					return err
				}
				for node, ip := range tmpIPMap {
					publicIPMap[node] = ip
				}
			} else {
				for i, node := range cloudConfigMap[region].InstanceIDs {
					publicIPMap[node] = cloudConfigMap[region].PublicIPs[i]
				}
			}
			if separateMonitoringInstance {
				if err = AddMonitoringSecurityGroupRule(ec2SvcMap, monitoringNodeConfig.PublicIPs[0], cloudConfigMap[region].SecurityGroup, region); err != nil {
					return err
				}
			}
		}
	} else {
		if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.GCPCloudService) != nil) {
			return fmt.Errorf("cloud access is required")
		}
		// Get GCP Credential, zone, Image ID, service account key file path, and GCP project name
		gcpClient, zones, numNodes, imageID, credentialFilepath, projectName, err := getGCPConfig()
		if err != nil {
			return err
		}
		if existingMonitoringInstance == "" {
			monitoringHostRegion = zones[0]
		}
		if !skipMonitoring {
			setUpMonitoring, separateMonitoringInstance, err = promptSetUpMonitoring()
			if err != nil {
				return err
			}
		}
		cloudConfigMap, err = createGCPInstance(usr, gcpClient, nodeType, numNodes, zones, imageID, clusterName, false)
		if err != nil {
			return err
		}
		if separateMonitoringInstance && existingMonitoringInstance == "" {
			monitoringCloudConfig, err := createGCPInstance(usr, gcpClient, nodeType, []int{1}, []string{monitoringHostRegion}, imageID, clusterName, true)
			if err != nil {
				return err
			}
			monitoringNodeConfig = monitoringCloudConfig[zones[0]]
		}
		if existingMonitoringInstance != "" {
			separateMonitoringInstance = true
			monitoringNodeConfig, monitoringHostRegion, err = getNodeCloudConfig(existingMonitoringInstance)
			if err != nil {
				return err
			}
		}
		if !useStaticIP && separateMonitoringInstance {
			monitoringPublicIPMap, err := gcpClient.GetInstancePublicIPs(monitoringHostRegion, monitoringNodeConfig.InstanceIDs)
			if err != nil {
				return err
			}
			monitoringNodeConfig.PublicIPs = []string{monitoringPublicIPMap[monitoringNodeConfig.InstanceIDs[0]]}
		}
		for _, zone := range zones {
			if !useStaticIP {
				tmpIPMap, err := gcpClient.GetInstancePublicIPs(zone, cloudConfigMap[zone].InstanceIDs)
				if err != nil {
					return err
				}
				for node, ip := range tmpIPMap {
					publicIPMap[node] = ip
				}
			} else {
				for i, node := range cloudConfigMap[zone].InstanceIDs {
					publicIPMap[node] = cloudConfigMap[zone].PublicIPs[i]
				}
			}
			if separateMonitoringInstance {
				networkName := fmt.Sprintf("%s-network", usr.Username+constants.AvalancheCLISuffix)
				firewallName := fmt.Sprintf("%s-%s-monitoring", networkName, strings.ReplaceAll(monitoringNodeConfig.PublicIPs[0], ".", ""))
				ports := []string{
					strconv.Itoa(constants.AvalanchegoMachineMetricsPort), strconv.Itoa(constants.AvalanchegoAPIPort),
					strconv.Itoa(constants.AvalanchegoMonitoringPort), strconv.Itoa(constants.AvalanchegoGrafanaPort),
				}
				if err = gcpClient.AddFirewall(
					monitoringNodeConfig.PublicIPs[0],
					networkName,
					projectName,
					firewallName,
					ports,
					true); err != nil {
					return err
				}
			}
		}
		gcpProjectName = projectName
		gcpCredentialFilepath = credentialFilepath
	}
	if err = createClusterNodeConfig(network, cloudConfigMap, monitoringNodeConfig, monitoringHostRegion, clusterName, cloudService, separateMonitoringInstance); err != nil {
		return err
	}
	if cloudService == constants.GCPCloudService {
		if err = updateClustersConfigGCPKeyFilepath(gcpProjectName, gcpCredentialFilepath); err != nil {
			return err
		}
	}

	inventoryPath := app.GetAnsibleInventoryDirPath(clusterName)
	avalancheGoVersion, err := getAvalancheGoVersion()
	if err != nil {
		return err
	}
	if err = ansible.CreateAnsibleHostInventory(inventoryPath, "", cloudService, publicIPMap, cloudConfigMap); err != nil {
		return err
	}
	monitoringInventoryPath := ""
	var monitoringHosts []*models.Host
	if separateMonitoringInstance {
		monitoringInventoryPath = filepath.Join(app.GetAnsibleInventoryDirPath(clusterName), constants.MonitoringDir)
		if existingMonitoringInstance == "" {
			if err = ansible.CreateAnsibleHostInventory(monitoringInventoryPath, monitoringNodeConfig.CertFilePath, cloudService, map[string]string{monitoringNodeConfig.InstanceIDs[0]: monitoringNodeConfig.PublicIPs[0]}, nil); err != nil {
				return err
			}
		}
		monitoringHosts, err = ansible.GetInventoryFromAnsibleInventoryFile(monitoringInventoryPath)
		if err != nil {
			return err
		}
	}
	allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(inventoryPath)
	if err != nil {
		return err
	}
	hosts := utils.Filter(allHosts, func(h *models.Host) bool { return slices.Contains(cloudConfigMap.GetAllInstanceIDs(), h.GetCloudID()) })
	// waiting for all nodes to become accessible
	failedHosts := waitForHosts(hosts)
	if failedHosts.Len() > 0 {
		for _, result := range failedHosts.GetResults() {
			ux.Logger.PrintToUser("Instance %s failed to provision with error %s. Please check instance logs for more information", result.NodeID, result.Err)
		}
		return fmt.Errorf("failed to provision node(s) %s", failedHosts.GetNodeList())
	}
	ux.Logger.PrintToUser("Installing AvalancheGo and Avalanche-CLI and starting bootstrap process on the newly created Avalanche node(s) ...")
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if err := host.Connect(); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			if err := provideStakingCertAndKey(host); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			if err := ssh.RunSSHSetupNode(host, app.Conf.GetConfigPath(), avalancheGoVersion, network.Kind == models.Devnet); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			if separateMonitoringInstance {
				if err := ssh.RunSSHSetupMachineMetrics(host); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
					return
				}
			} else {
				if setUpMonitoring {
					if err := ssh.RunSSHSetupMonitoring(host); err != nil {
						nodeResults.AddResult(host.NodeID, nil, err)
						return
					}
				}
			}
			if err := ssh.RunSSHSetupBuildEnv(host); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			if err := ssh.RunSSHSetupCLIFromSource(host, constants.SetupCLIFromSourceBranch); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
		}(&wgResults, host)
	}
	wg.Wait()
	ansibleHostIDs, err := utils.MapWithError(cloudConfigMap.GetAllInstanceIDs(), func(s string) (string, error) { return models.HostCloudIDToAnsibleID(cloudService, s) })
	if err != nil {
		return err
	}
	if separateMonitoringInstance {
		if len(monitoringHosts) != 1 {
			return fmt.Errorf("expected only one monitoring host, found %d", len(monitoringHosts))
		}
		monitoringHost := monitoringHosts[0]
		// remove monitoring host from created hosts list
		hosts = utils.Filter(hosts, func(h *models.Host) bool { return h.NodeID != monitoringHost.NodeID })
		avalancheGoPorts := []string{}
		machinePorts := []string{}
		inventoryHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
		if err != nil {
			return err
		}
		for _, host := range inventoryHosts {
			avalancheGoPorts = append(avalancheGoPorts, fmt.Sprintf("'%s:%s'", host.IP, strconv.Itoa(constants.AvalanchegoAPIPort)))
			machinePorts = append(machinePorts, fmt.Sprintf("'%s:%s'", host.IP, strconv.Itoa(constants.AvalanchegoMachineMetricsPort)))
		}
		if existingMonitoringInstance != "" {
			if err := ssh.RunSSHUpdatePrometheusConfig(monitoringHost, strings.Join(avalancheGoPorts, ","), strings.Join(machinePorts, ",")); err != nil {
				return err
			}
		} else {
			if err = app.SetupMonitoringEnv(); err != nil {
				return err
			}
			if err := ssh.RunSSHCopyMonitoringDashboards(monitoringHost, app.GetMonitoringDashboardDir()+"/"); err != nil {
				return err
			}
			if err := ssh.RunSSHSetupSeparateMonitoring(monitoringHost, app.GetMonitoringScriptFile(), strings.Join(avalancheGoPorts, ","), strings.Join(machinePorts, ",")); err != nil {
				return err
			}
		}
		for _, ansibleNodeID := range ansibleHostIDs {
			if err = app.CreateAnsibleNodeConfigDir(ansibleNodeID); err != nil {
				return err
			}
		}
		// download node configs
		wg := sync.WaitGroup{}
		wgResults := models.NodeResults{}
		for _, host := range hosts {
			wg.Add(1)
			go func(nodeResults *models.NodeResults, host *models.Host) {
				defer wg.Done()
				nodeDirPath := app.GetNodeInstanceAvaGoConfigDirPath(host.NodeID)
				if err := ssh.RunSSHDownloadNodeMonitoringConfig(host, nodeDirPath); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
					return
				}
				if err = addHTTPHostToConfigFile(app.GetNodeConfigJSONFile(host.NodeID)); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
					return
				}
				if err := ssh.RunSSHUploadNodeMonitoringConfig(host, nodeDirPath); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
					return
				}
				if err := ssh.RunSSHRestartNode(host); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
					return
				}
				if err := os.RemoveAll(nodeDirPath); err != nil {
					return
				}
			}(&wgResults, host)
		}
		wg.Wait()
		for _, node := range hosts {
			if wgResults.HasNodeIDWithError(node.NodeID) {
				ux.Logger.PrintToUser("Node %s is ERROR with error: %s", node.NodeID, wgResults.GetErrorHostMap()[node.NodeID])
				return fmt.Errorf("node %s failed to setup with error: %w", node.NodeID, wgResults.GetErrorHostMap()[node.NodeID])
			}
		}
	}
	ux.Logger.PrintToUser("======================================")
	ux.Logger.PrintToUser("AVALANCHE NODE(S) STATUS")
	ux.Logger.PrintToUser("======================================")
	ux.Logger.PrintToUser("")
	for _, node := range hosts {
		if wgResults.HasNodeIDWithError(node.NodeID) {
			ux.Logger.PrintToUser("Node %s is ERROR with error: %s", node.NodeID, wgResults.GetErrorHostMap()[node.NodeID])
		} else {
			ux.Logger.PrintToUser("Node %s is CREATED", node.NodeID)
		}
	}
	if network.Kind == models.Devnet {
		ux.Logger.PrintToUser("Setting up Devnet ...")
		if err := setupDevnet(clusterName, hosts); err != nil {
			return err
		}
	}

	if wgResults.HasErrors() {
		return fmt.Errorf("failed to deploy node(s) %s", wgResults.GetErrorHostMap())
	} else {
		monitoringPublicIP := ""
		if separateMonitoringInstance {
			monitoringPublicIP = monitoringNodeConfig.PublicIPs[0]
		}
		printResults(cloudConfigMap, publicIPMap, ansibleHostIDs, monitoringPublicIP)
		ux.Logger.PrintToUser("AvalancheGo and Avalanche-CLI installed and node(s) are bootstrapping!")
	}
	return nil
}

func promptSetUpMonitoring() (bool, bool, error) {
	var err error
	if !separateMonitoringInstance && existingMonitoringInstance == "" {
		if sameMonitoringInstance {
			return true, false, nil
		}
		setUpMonitoring, err = app.Prompt.CaptureYesNo("Do you want to set up monitoring for your instances? (This enables you to monitor validator and machine metrics)")
		if err != nil {
			return false, false, err
		}
		if setUpMonitoring {
			separateMonitoringInstance, err = app.Prompt.CaptureYesNo("Do you want to set up a separate instance to host monitoring? (This enables you to monitor all your set up instances in one dashboard)")
			if err != nil {
				return false, false, err
			}
		}
		return setUpMonitoring, separateMonitoringInstance, nil
	}
	return setUpMonitoring, separateMonitoringInstance, nil
}

// createClusterNodeConfig creates node config and save it in .avalanche-cli/nodes/{instanceID}
// also creates cluster config in .avalanche-cli/nodes storing various key pair and security group info for all clusters
func createClusterNodeConfig(network models.Network, cloudConfigMap models.CloudConfig, monitorCloudConfig models.RegionConfig, monitoringHostRegion, clusterName, cloudService string, separateMonitoringInstance bool) error {
	for region, cloudConfig := range cloudConfigMap {
		for i := range cloudConfig.InstanceIDs {
			publicIP := ""
			if len(cloudConfig.PublicIPs) > 0 {
				publicIP = cloudConfig.PublicIPs[i]
			}
			nodeConfig := models.NodeConfig{
				NodeID:        cloudConfig.InstanceIDs[i],
				Region:        region,
				AMI:           cloudConfig.ImageID,
				KeyPair:       cloudConfig.KeyPair,
				CertPath:      cloudConfig.CertFilePath,
				SecurityGroup: cloudConfig.SecurityGroup,
				ElasticIP:     publicIP,
				CloudService:  cloudService,
				UseStaticIP:   useStaticIP,
			}
			err := app.CreateNodeCloudConfigFile(cloudConfig.InstanceIDs[i], &nodeConfig)
			if err != nil {
				return err
			}
			if err = addNodeToClustersConfig(network, cloudConfig.InstanceIDs[i], clusterName, false); err != nil {
				return err
			}
		}
		if separateMonitoringInstance {
			publicIP := ""
			if useStaticIP {
				publicIP = monitorCloudConfig.PublicIPs[0]
			}
			nodeConfig := models.NodeConfig{
				NodeID:        monitorCloudConfig.InstanceIDs[0],
				Region:        monitoringHostRegion,
				AMI:           monitorCloudConfig.ImageID,
				KeyPair:       monitorCloudConfig.KeyPair,
				CertPath:      monitorCloudConfig.CertFilePath,
				SecurityGroup: monitorCloudConfig.SecurityGroup,
				ElasticIP:     publicIP,
				CloudService:  cloudService,
			}
			if err := app.CreateNodeCloudConfigFile(monitorCloudConfig.InstanceIDs[0], &nodeConfig); err != nil {
				return err
			}
			if err := addNodeToClustersConfig(network, monitorCloudConfig.InstanceIDs[0], clusterName, true); err != nil {
				return err
			}
			if err := updateKeyPairClustersConfig(nodeConfig); err != nil {
				return err
			}
		}
	}
	return nil
}

func addHTTPHostToConfigFile(filePath string) error {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	var result map[string]interface{}
	if err := json.Unmarshal(byteValue, &result); err != nil {
		return err
	}
	result["http-host"] = "0.0.0.0"
	byteValue, err = json.MarshalIndent(result, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, byteValue, constants.WriteReadReadPerms)
}

func getExistingMonitoringInstance(clusterName string) (string, error) {
	if app.ClustersConfigExists() {
		clustersConfig, err := app.LoadClustersConfig()
		if err != nil {
			return "", err
		}
		if _, ok := clustersConfig.Clusters[clusterName]; ok {
			if clustersConfig.Clusters[clusterName].MonitoringInstance != "" {
				return clustersConfig.Clusters[clusterName].MonitoringInstance, nil
			}
		}
	}
	return "", nil
}

func updateKeyPairClustersConfig(cloudConfig models.NodeConfig) error {
	clustersConfig := models.ClustersConfig{}
	var err error
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if clustersConfig.KeyPair == nil {
		clustersConfig.KeyPair = make(map[string]string)
	}
	if _, ok := clustersConfig.KeyPair[cloudConfig.KeyPair]; !ok {
		clustersConfig.KeyPair[cloudConfig.KeyPair] = cloudConfig.CertPath
	}
	return app.WriteClustersConfigFile(&clustersConfig)
}

func getNodeCloudConfig(node string) (models.RegionConfig, string, error) {
	config, err := app.LoadClusterNodeConfig(node)
	if err != nil {
		return models.RegionConfig{}, "", err
	}
	elasticIP := []string{}
	if config.ElasticIP != "" {
		elasticIP = append(elasticIP, config.ElasticIP)
	}
	instanceIDs := []string{}
	instanceIDs = append(instanceIDs, config.NodeID)
	return models.RegionConfig{
		InstanceIDs:       instanceIDs,
		PublicIPs:         elasticIP,
		KeyPair:           config.KeyPair,
		SecurityGroupName: config.SecurityGroup,
		CertFilePath:      config.CertPath,
		ImageID:           config.AMI,
	}, config.Region, nil
}

func addNodeToClustersConfig(network models.Network, nodeID, clusterName string, isMonitoringInstance bool) error {
	clustersConfig := models.ClustersConfig{}
	var err error
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if clustersConfig.Clusters == nil {
		clustersConfig.Clusters = make(map[string]models.ClusterConfig)
	}
	if _, ok := clustersConfig.Clusters[clusterName]; !ok {
		clustersConfig.Clusters[clusterName] = models.ClusterConfig{
			Network: network,
			Nodes:   []string{},
		}
	}
	nodes := clustersConfig.Clusters[clusterName].Nodes
	if !isMonitoringInstance {
		// monitoring instance will always be last in the loop, so no need to set monitoring instance here
		clustersConfig.Clusters[clusterName] = models.ClusterConfig{
			Network: network,
			Nodes:   append(nodes, nodeID),
		}
	} else {
		clustersConfig.Clusters[clusterName] = models.ClusterConfig{
			Network:            network,
			Nodes:              nodes,
			MonitoringInstance: nodeID,
		}
	}

	return app.WriteClustersConfigFile(&clustersConfig)
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

func provideStakingCertAndKey(host *models.Host) error {
	instanceID := host.GetCloudID()
	keyPath := filepath.Join(app.GetNodesDir(), instanceID)
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
	return ssh.RunSSHUploadStakingFiles(host, keyPath)
}

func getIPAddress() (string, error) {
	resp, err := http.Get("https://api.ipify.org?format=json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("HTTP request failed")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
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
	} else if useCustomAvalanchegoVersion != "" {
		if !semver.IsValid(useCustomAvalanchegoVersion) {
			return "", errors.New("custom avalanchego version must be a legal semantic version (ex: v1.1.1)")
		}
		version = useCustomAvalanchegoVersion
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
			customVersion, err := app.Prompt.CaptureVersion("Which version of AvalancheGo would you like to install? (Use format v1.10.13)")
			if err != nil {
				return "", err
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
			_, err = subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName})
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

func setCloudInstanceType(cloudService string) (string, error) {
	switch { // backwards compatibility
	case nodeType == "default" && cloudService == constants.AWSCloudService:
		nodeType = constants.AWSDefaultInstanceType
		return nodeType, nil
	case nodeType == "default" && cloudService == constants.GCPCloudService:
		nodeType = constants.GCPDefaultInstanceType
		return nodeType, nil
	}
	defaultNodeType := ""
	nodeTypeOption2 := ""
	nodeTypeOption3 := ""
	customNodeType := "Choose custom instance type"
	switch {
	case cloudService == constants.AWSCloudService:
		defaultNodeType = constants.AWSDefaultInstanceType
		nodeTypeOption2 = "t3a.2xlarge" // burst
		nodeTypeOption3 = "c5n.2xlarge"
	case cloudService == constants.GCPCloudService:
		defaultNodeType = constants.GCPDefaultInstanceType
		nodeTypeOption2 = "c3-highcpu-8"
		nodeTypeOption3 = "n2-standard-8"
	}
	if nodeType == "" {
		defaultStr := "[default] (recommended)"
		nodeTypeStr, err := app.Prompt.CaptureList(
			"Instance type to use",
			[]string{fmt.Sprintf("%s %s", defaultNodeType, defaultStr), nodeTypeOption2, nodeTypeOption3, customNodeType},
		)
		if err != nil {
			ux.Logger.PrintToUser("Failed to capture node type with error: %s", err.Error())
			return "", err
		}
		nodeTypeStr = strings.ReplaceAll(nodeTypeStr, defaultStr, "") // remove (default) if any
		if nodeTypeStr == customNodeType {
			nodeTypeStr, err = app.Prompt.CaptureString("What instance type would you like to use? Please refer to https://docs.avax.network/nodes/run/node-manually#hardware-and-os-requirements for minimum hardware requirements")
			if err != nil {
				ux.Logger.PrintToUser("Failed to capture custom node type with error: %s", err.Error())
				return "", err
			}
		}
		return strings.Trim(nodeTypeStr, " "), nil
	}
	return nodeType, nil
}

func printResults(cloudConfigMap models.CloudConfig, publicIPMap map[string]string, ansibleHostIDs []string, monitoringHostIP string) {
	ux.Logger.PrintToUser("======================================")
	ux.Logger.PrintToUser("AVALANCHE NODE(S) SUCCESSFULLY SET UP!")
	ux.Logger.PrintToUser("======================================")
	ux.Logger.PrintToUser("Please wait until the node(s) are successfully bootstrapped to run further commands on the node(s)")
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Here are the details of the set up node(s): ")
	for region, cloudConfig := range cloudConfigMap {
		ux.Logger.PrintToUser(fmt.Sprintf("Don't delete or replace your ssh private key file at %s as you won't be able to access your cloud server without it", cloudConfig.CertFilePath))
		for i, instanceID := range cloudConfig.InstanceIDs {
			publicIP := ""
			publicIP = publicIPMap[instanceID]
			ux.Logger.PrintToUser("======================================")
			ux.Logger.PrintToUser(fmt.Sprintf("Node %s details: ", ansibleHostIDs[i]))
			ux.Logger.PrintToUser(fmt.Sprintf("Cloud Instance ID: %s", instanceID))
			ux.Logger.PrintToUser(fmt.Sprintf("Public IP: %s", publicIP))
			ux.Logger.PrintToUser(fmt.Sprintf("Cloud Region: %s", region))
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser(fmt.Sprintf("staker.crt and staker.key are stored at %s. If anything happens to your node or the machine node runs on, these files can be used to fully recreate your node.", app.GetNodeInstanceDirPath(instanceID)))
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser("To ssh to node, run: ")
			ux.Logger.PrintToUser("")
			ux.Logger.PrintToUser(utils.GetSSHConnectionString(publicIP, cloudConfig.CertFilePath))
			ux.Logger.PrintToUser("")
			if setUpMonitoring && !separateMonitoringInstance {
				ux.Logger.PrintToUser("To view monitoring dashboard for this node, visit the following link in your browser: ")
				ux.Logger.PrintToUser(fmt.Sprintf("http://%s:3000/dashboards", publicIP))
				ux.Logger.PrintToUser("Log in with username: admin, password: admin")
				ux.Logger.PrintToUser("")
			}
			ux.Logger.PrintToUser("======================================")
		}
	}
	if separateMonitoringInstance {
		ux.Logger.PrintToUser("")
		ux.Logger.PrintToUser("To view unified node monitoring dashboard, visit the following link in your browser: ")
		ux.Logger.PrintToUser(fmt.Sprintf("http://%s:3000/dashboards", monitoringHostIP))
		ux.Logger.PrintToUser("Log in with username: admin, password: admin")
		ux.Logger.PrintToUser("")
	}
	ux.Logger.PrintToUser("")
}

// waitForHosts waits for all hosts to become available via SSH.
func waitForHosts(hosts []*models.Host) *models.NodeResults {
	hostErrors := models.NodeResults{}
	createdWaitGroup := sync.WaitGroup{}
	for _, host := range hosts {
		createdWaitGroup.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer createdWaitGroup.Done()
			if err := host.WaitForSSHShell(constants.SSHServerStartTimeout); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
		}(&hostErrors, host)
	}
	createdWaitGroup.Wait()
	return &hostErrors
}

// requestCloudAuth makes sure user agree to
func requestCloudAuth(cloudName string) error {
	ux.Logger.PrintToUser("Do you authorize Avalanche-CLI to access your %s account?", cloudName)
	ux.Logger.PrintToUser("By clicking yes, you are authorizing Avalanche-CLI to:")
	ux.Logger.PrintToUser("- Create Cloud instance(s) and other components (such as elastic IPs)")
	ux.Logger.PrintToUser("- Start/Stop Cloud instance(s) and other components (such as elastic IPs) previously created by Avalanche-CLI")
	ux.Logger.PrintToUser("- Delete Cloud instance(s) and other components (such as elastic IPs) previously created by Avalanche-CLI")
	yes, err := app.Prompt.CaptureYesNo(fmt.Sprintf("I authorize Avalanche-CLI to access my %s account", cloudName))
	if err != nil {
		return err
	}
	if err := app.Conf.SetConfigValue(constants.ConfigAutorizeCloudAccessKey, yes); err != nil {
		return err
	}
	if !yes {
		return fmt.Errorf("user did not give authorization to Avalanche-CLI to access %s account", cloudName)
	}
	return nil
}

func getRegionsNodeNum(cloudName string) (
	map[string]int,
	error,
) {
	type CloudPrompt struct {
		defaultLocations []string
		locationName     string
		locationsListURL string
	}

	supportedClouds := map[string]CloudPrompt{
		constants.AWSCloudService: {
			defaultLocations: []string{"us-east-1", "us-east-2", "us-west-1", "us-west-2"},
			locationName:     "AWS Region",
			locationsListURL: "https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html",
		},
		constants.GCPCloudService: {
			defaultLocations: []string{"us-east1-b", "us-central1-c", "us-west1-b"},
			locationName:     "Google Zone",
			locationsListURL: "https://cloud.google.com/compute/docs/regions-zones/",
		},
	}

	if _, ok := supportedClouds[cloudName]; !ok {
		return nil, fmt.Errorf("cloud %s is not supported", cloudName)
	}

	nodes := map[string]int{}
	awsCustomRegion := fmt.Sprintf("Choose custom %s (list of %ss available at %s)", supportedClouds[cloudName].locationName, supportedClouds[cloudName].locationName, supportedClouds[cloudName].locationsListURL)
	additionalRegionPrompt := fmt.Sprintf("Would you like to add additional %s?", supportedClouds[cloudName].locationName)
	for {
		userRegion, err := app.Prompt.CaptureList(
			fmt.Sprintf("Which %s do you want to set up your node(s) in?", supportedClouds[cloudName].locationName),
			append(supportedClouds[cloudName].defaultLocations, awsCustomRegion),
		)
		if err != nil {
			return nil, err
		}
		if userRegion == awsCustomRegion {
			userRegion, err = app.Prompt.CaptureString(fmt.Sprintf("Which %s do you want to set up your node in?", supportedClouds[cloudName].locationName))
			if err != nil {
				return nil, err
			}
		}
		numNodes, err := app.Prompt.CaptureUint32(fmt.Sprintf("How many nodes do you want to set up in %s %s?", userRegion, supportedClouds[cloudName].locationName))
		if err != nil {
			return nil, err
		}
		if numNodes > uint32(math.MaxInt32) {
			return nil, fmt.Errorf("number of nodes exceeds the range of a signed 32-bit integer")
		}
		nodes[userRegion] = int(numNodes)

		currentInput := utils.Map(maps.Keys(nodes), func(region string) string { return fmt.Sprintf("[%s]:%d", region, nodes[region]) })
		ux.Logger.PrintToUser("Current selection: " + strings.Join(currentInput, " "))
		yes, err := app.Prompt.CaptureNoYes(additionalRegionPrompt)
		if err != nil {
			return nil, err
		}
		if !yes {
			return nodes, nil
		}
	}
}

func setSSHIdentity() (string, error) {
	const yubikeyMark = " [YubiKey] (recommended)"
	const yubikeyPattern = `cardno:(\d+(_\d+)*)`
	sshIdentities, err := utils.ListSSHAgentIdentities()
	if err != nil {
		return "", err
	}
	yubikeyRegexp := regexp.MustCompile(yubikeyPattern)
	sshIdentities = utils.Map(sshIdentities, func(id string) string {
		if len(yubikeyRegexp.FindStringSubmatch(id)) > 0 {
			return fmt.Sprintf("%s%s", id, yubikeyMark)
		}
		return id
	})
	sshIdentity, err := app.Prompt.CaptureList(
		"Which SSH identity do you want to use?", sshIdentities,
	)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(sshIdentity, yubikeyMark, ""), nil
}
