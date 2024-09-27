// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"golang.org/x/exp/slices"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"

	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
)

func getServiceAccountKeyFilepath() (string, error) {
	if cmdLineGCPCredentialsPath != "" {
		return cmdLineGCPCredentialsPath, nil
	}
	ux.Logger.PrintToUser("To create a VM instance in GCP, you can use your account credentials")
	ux.Logger.PrintToUser("Please follow instructions detailed at https://developers.google.com/workspace/guides/create-credentials#service-account to set up a GCP service account")
	ux.Logger.PrintToUser("Or use https://cloud.google.com/sdk/docs/authorizing#user-account for authorization without a service account")
	customAuthKeyPath := "Choose custom path for credentials JSON file"
	credJSONFilePath, err := app.Prompt.CaptureList(
		"What is the filepath to the credentials JSON file?",
		[]string{constants.GCPDefaultAuthKeyPath, customAuthKeyPath},
	)
	if err != nil {
		return "", err
	}
	if credJSONFilePath == customAuthKeyPath {
		credJSONFilePath, err = app.Prompt.CaptureString("What is the custom filepath to the credentials JSON file?")
		if err != nil {
			return "", err
		}
	}
	return utils.GetRealFilePath(credJSONFilePath), err
}

func getGCPCloudCredentials() (*compute.Service, string, string, error) {
	var err error
	var gcpCredentialsPath string
	var gcpProjectName string
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return nil, "", "", err
		}
		if clustersConfig.GCPConfig != (models.GCPConfig{}) {
			gcpProjectName = clustersConfig.GCPConfig.ProjectName
			gcpCredentialsPath = clustersConfig.GCPConfig.ServiceAccFilePath
		}
	}
	if gcpProjectName == "" {
		if cmdLineGCPProjectName != "" {
			gcpProjectName = cmdLineGCPProjectName
		} else {
			gcpProjectName, err = app.Prompt.CaptureString("What is the name of your Google Cloud project?")
			if err != nil {
				return nil, "", "", err
			}
		}
	}
	if gcpCredentialsPath == "" {
		gcpCredentialsPath, err = getServiceAccountKeyFilepath()
		if err != nil {
			return nil, "", "", err
		}
	}
	err = os.Setenv(constants.GCPEnvVar, gcpCredentialsPath)
	if err != nil {
		return nil, "", "", err
	}
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		return nil, "", "", err
	}
	computeService, err := compute.New(client)
	return computeService, gcpProjectName, gcpCredentialsPath, err
}

func getGCPConfig(singleNode bool) (*gcpAPI.GcpCloud, map[string]NumNodes, string, string, string, error) {
	finalRegions := map[string]NumNodes{}
	switch {
	case len(numValidatorsNodes) != len(utils.Unique(cmdLineRegion)):
		return nil, nil, "", "", "", errors.New("number of regions and number of nodes must be equal. Please make sure list of regions is unique")
	case len(cmdLineRegion) == 0 && len(numValidatorsNodes) == 0:
		var err error
		if singleNode {
			selectedRegion, err := getSeparateHostNodeParam(constants.GCPCloudService)
			finalRegions = map[string]NumNodes{selectedRegion: {1, 0}}
			if err != nil {
				return nil, nil, "", "", "", err
			}
		} else {
			finalRegions, err = getRegionsNodeNum(constants.GCPCloudService)
			if err != nil {
				return nil, nil, "", "", "", err
			}
		}
	default:
		if globalNetworkFlags.UseDevnet || globalNetworkFlags.UseFuji {
			for i, region := range cmdLineRegion {
				finalRegions[region] = NumNodes{numValidatorsNodes[i], numAPINodes[i]}
			}
		} else {
			for i, region := range cmdLineRegion {
				finalRegions[region] = NumNodes{numValidatorsNodes[i], 0}
			}
		}
	}
	gcpClient, projectName, gcpCredentialFilePath, err := getGCPCloudCredentials()
	if err != nil {
		return nil, nil, "", "", "", err
	}
	gcpCloud, err := gcpAPI.NewGcpCloud(gcpClient, projectName, context.Background())
	if err != nil {
		return nil, nil, "", "", "", err
	}
	finalZones := map[string]NumNodes{}
	// verify regions are valid and place in random zones per region
	for region, numNodes := range finalRegions {
		if !slices.Contains(gcpCloud.ListRegions(), region) {
			return nil, nil, "", "", "", fmt.Errorf("invalid region %s", region)
		} else {
			finalZone, err := gcpCloud.GetRandomZone(region)
			if err != nil {
				return nil, nil, "", "", "", err
			}
			finalZones[finalZone] = numNodes
		}
	}
	imageID, err := gcpCloud.GetUbuntuImageID()
	if err != nil {
		return nil, nil, "", "", "", err
	}
	return gcpCloud, finalZones, imageID, gcpCredentialFilePath, projectName, nil
}

// createGCEInstances creates Google Compute Engine VM instances
func createGCEInstances(gcpClient *gcpAPI.GcpCloud,
	instanceType string,
	numNodesMap map[string]NumNodes,
	ami,
	cliDefaultName string,
	forMonitoring bool,
) (map[string][]string, map[string][]string, string, string, error) {
	keyPairName := fmt.Sprintf("%s-keypair", cliDefaultName)
	sshKeyPath, err := app.GetSSHCertFilePath(keyPairName)
	if err != nil {
		return nil, nil, "", "", err
	}
	networkName := fmt.Sprintf("%s-network", cliDefaultName)
	if !forMonitoring {
		ux.Logger.PrintToUser("Creating new VM instance(s) on Google Compute Engine...")
	} else {
		ux.Logger.PrintToUser("Creating separate monitoring VM instance(s) on Google Compute Engine...")
	}
	certInSSHDir, err := app.CheckCertInSSHDir(fmt.Sprintf("%s-keypair.pub", cliDefaultName))
	if err != nil {
		return nil, nil, "", "", err
	}
	if !useSSHAgent && !certInSSHDir {
		ux.Logger.PrintToUser("Creating new SSH key pair %s in GCP", sshKeyPath)
		ux.Logger.PrintToUser("For more information regarding SSH key pair in GCP, please head to https://cloud.google.com/compute/docs/connect/create-ssh-keys")
		_, err = exec.Command("ssh-keygen", "-t", "rsa", "-f", sshKeyPath, "-C", "ubuntu", "-b", "2048").Output()
		if err != nil {
			return nil, nil, "", "", err
		}
	}

	networkExists, err := gcpClient.CheckNetworkExists(networkName)
	if err != nil {
		return nil, nil, "", "", err
	}
	userIPAddress, err := utils.GetUserIPAddress()
	if err != nil {
		return nil, nil, "", "", err
	}
	if !networkExists {
		ux.Logger.PrintToUser("Creating new network %s in GCP", networkName)
		if _, err := gcpClient.SetupNetwork(userIPAddress, networkName); err != nil {
			return nil, nil, "", "", err
		}
	} else {
		ux.Logger.PrintToUser("Using existing network %s in GCP", networkName)
		firewallName := fmt.Sprintf("%s-%s", networkName, strings.ReplaceAll(userIPAddress, ".", ""))
		firewallExists, err := gcpClient.CheckFirewallExists(firewallName, false)
		if err != nil {
			return nil, nil, "", "", err
		}
		if !firewallExists {
			_, err := gcpClient.SetFirewallRule(
				userIPAddress,
				firewallName,
				networkName,
				[]string{
					strconv.Itoa(constants.SSHTCPPort),
					strconv.Itoa(constants.AvalanchegoAPIPort),
					strconv.Itoa(constants.AvalanchegoMonitoringPort),
					strconv.Itoa(constants.AvalanchegoGrafanaPort),
				},
			)
			if err != nil {
				return nil, nil, "", "", err
			}
		} else {
			firewallMonitoringName := fmt.Sprintf("%s-monitoring", firewallName)
			// check that the separate monitoring firewall doesn't exist
			firewallExists, err = gcpClient.CheckFirewallExists(firewallMonitoringName, false)
			if err != nil {
				return nil, nil, "", "", err
			}
			if !firewallExists {
				_, err := gcpClient.SetFirewallRule(userIPAddress, firewallMonitoringName, networkName, []string{strconv.Itoa(constants.AvalanchegoMonitoringPort), strconv.Itoa(constants.AvalanchegoGrafanaPort)})
				if err != nil {
					return nil, nil, "", "", err
				}
			}
			firewallLoggingName := fmt.Sprintf("%s-logging", firewallName)
			firewallExists, err = gcpClient.CheckFirewallExists(firewallLoggingName, false)
			if err != nil {
				return nil, nil, "", "", err
			}
			if !firewallExists {
				_, err := gcpClient.SetFirewallRule("0.0.0.0/0", firewallLoggingName, networkName, []string{strconv.Itoa(constants.AvalanchegoLokiPort)})
				if err != nil {
					return nil, nil, "", "", err
				}
			}
		}
	}
	nodeName := map[string]string{}
	for zone := range numNodesMap {
		nodeName[zone] = utils.RandomString(5)
	}
	publicIP := map[string][]string{}
	if useStaticIP {
		for zone, numNodes := range numNodesMap {
			publicIP[zone], err = gcpClient.SetPublicIP(zone, nodeName[zone], numNodes.All())
			if err != nil {
				return nil, nil, "", "", err
			}
		}
	}
	sshPublicKey := ""
	if useSSHAgent {
		sshPublicKey, err = utils.ReadSSHAgentIdentityPublicKey(sshIdentity)
		if err != nil {
			return nil, nil, "", "", err
		}
	} else {
		sshPublicKeyBytes, err := os.ReadFile(fmt.Sprintf("%s.pub", sshKeyPath))
		if err != nil {
			return nil, nil, "", "", err
		}
		sshPublicKey = string(sshPublicKeyBytes)
	}
	spinSession := ux.NewUserSpinner()
	for zone, numNodes := range numNodesMap {
		spinner := spinSession.SpinToUser("Waiting for instance(s) in GCP[%s] to be provisioned...", zone)
		_, err := gcpClient.SetupInstances(
			cliDefaultName,
			zone,
			networkName,
			sshPublicKey,
			ami,
			nodeName[zone],
			instanceType,
			publicIP[zone],
			numNodes.All(),
			forMonitoring)
		if err != nil {
			ux.SpinFailWithError(spinner, "", err)
			return nil, nil, "", "", err
		}
		ux.SpinComplete(spinner)
	}
	spinSession.Stop()
	instanceIDs := map[string][]string{}
	for zone, numNodes := range numNodesMap {
		instanceIDs[zone] = []string{}
		for i := 0; i < numNodes.All(); i++ {
			instanceIDs[zone] = append(instanceIDs[zone], fmt.Sprintf("%s-%s", nodeName[zone], strconv.Itoa(i)))
		}
	}
	ux.Logger.GreenCheckmarkToUser("New Compute instance(s) successfully created in GCP!")
	sshCertPath := ""
	if !useSSHAgent {
		sshCertPath, err = app.GetSSHCertFilePath(fmt.Sprintf("%s-keypair", cliDefaultName))
		if err != nil {
			return nil, nil, "", "", err
		}
	}
	return instanceIDs, publicIP, sshCertPath, keyPairName, nil
}

func createGCPInstance(
	gcpClient *gcpAPI.GcpCloud,
	instanceType string,
	numNodesMap map[string]NumNodes,
	imageID string,
	clusterName string,
	forMonitoring bool,
) (models.CloudConfig, error) {
	prefix, err := defaultAvalancheCLIPrefix("")
	if err != nil {
		return models.CloudConfig{}, err
	}
	for zoneToCheck := range numNodesMap {
		isSupported, err := gcpClient.IsInstanceTypeSupported(instanceType, zoneToCheck)
		if err != nil {
			return models.CloudConfig{}, err
		} else if !isSupported {
			return models.CloudConfig{}, fmt.Errorf("instance type %s is not supported in %s zone", instanceType, zoneToCheck)
		}
	}
	instanceIDs, elasticIPs, certFilePath, keyPairName, err := createGCEInstances(
		gcpClient,
		instanceType,
		numNodesMap,
		imageID,
		prefix,
		forMonitoring,
	)
	if err != nil {
		ux.Logger.PrintToUser("Failed to create GCP cloud server")
		// we destroy created instances so that user doesn't pay for unused GCP instances
		ux.Logger.PrintToUser("Destroying all created GCP instances due to error to prevent charge for unused GCP instances...")
		failedNodes := map[string]error{}
		for zone, zoneInstances := range instanceIDs {
			for _, instanceID := range zoneInstances {
				nodeConfig := models.NodeConfig{
					NodeID: instanceID,
					Region: zone,
				}
				if destroyErr := gcpClient.DestroyGCPNode(nodeConfig, clusterName); destroyErr != nil {
					failedNodes[instanceID] = destroyErr
					continue
				}
				ux.Logger.PrintToUser(fmt.Sprintf("GCP cloud server instance %s destroyed in %s zone", instanceID, zone))
			}
		}
		if len(failedNodes) > 0 {
			ux.Logger.PrintToUser("Failed nodes: ")
			for node, err := range failedNodes {
				ux.Logger.PrintToUser(fmt.Sprintf("Failed to destroy node %s due to %s", node, err))
			}
			ux.Logger.PrintToUser("Destroy the above instance(s) on GCP console to prevent charges")
			return models.CloudConfig{}, fmt.Errorf("failed to destroy node(s) %s", failedNodes)
		}
		return models.CloudConfig{}, err
	}
	ccm := models.CloudConfig{}
	for zone := range numNodesMap {
		ccm[zone] = models.RegionConfig{
			InstanceIDs:   instanceIDs[zone],
			PublicIPs:     elasticIPs[zone],
			KeyPair:       keyPairName,
			SecurityGroup: fmt.Sprintf("%s-network", prefix),
			CertFilePath:  certFilePath,
			ImageID:       imageID,
		}
	}
	return ccm, nil
}

func updateClustersConfigGCPKeyFilepath(projectName, serviceAccountKeyFilepath string) error {
	clustersConfig := models.ClustersConfig{}
	var err error
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if projectName != "" {
		clustersConfig.GCPConfig.ProjectName = projectName
	}
	if serviceAccountKeyFilepath != "" {
		clustersConfig.GCPConfig.ServiceAccFilePath = serviceAccountKeyFilepath
	}
	return app.WriteClustersConfigFile(&clustersConfig)
}

func grantAccessToPublicIPViaFirewall(gcpClient *gcpAPI.GcpCloud, projectName string, publicIP string, label string) error {
	prefix, err := defaultAvalancheCLIPrefix("")
	if err != nil {
		return err
	}
	networkName := fmt.Sprintf("%s-network", prefix)
	firewallName := fmt.Sprintf("%s-%s-%s", networkName, strings.ReplaceAll(publicIP, ".", ""), label)
	ports := []string{
		strconv.Itoa(constants.AvalanchegoMachineMetricsPort), strconv.Itoa(constants.AvalanchegoAPIPort),
		strconv.Itoa(constants.AvalanchegoMonitoringPort), strconv.Itoa(constants.AvalanchegoGrafanaPort),
		strconv.Itoa(constants.AvalanchegoLokiPort),
	}
	if err = gcpClient.AddFirewall(
		publicIP,
		networkName,
		projectName,
		firewallName,
		ports,
		true); err != nil {
		return err
	}
	return nil
}

func setGCPAWMRelayerSecurityGroupRule(awmRelayerHost *models.Host) error {
	gcpClient, _, _, _, projectName, err := getGCPConfig(true)
	if err != nil {
		return err
	}
	prefix, err := defaultAvalancheCLIPrefix("")
	if err != nil {
		return err
	}
	networkName := fmt.Sprintf("%s-network", prefix)
	firewallName := fmt.Sprintf("%s-%s-relayer", networkName, strings.ReplaceAll(awmRelayerHost.IP, ".", ""))
	ports := []string{
		strconv.Itoa(constants.AvalanchegoAPIPort),
	}
	return gcpClient.AddFirewall(
		awmRelayerHost.IP,
		networkName,
		projectName,
		firewallName,
		ports,
		false,
	)
}
