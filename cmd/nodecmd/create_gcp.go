// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	terraformgcp "github.com/ava-labs/avalanche-cli/pkg/terraform/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/rand"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/ava-labs/avalanche-cli/pkg/models"

	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/terraform"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
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
	if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.GCPCloudService) != nil) {
		return nil, "", "", fmt.Errorf("cloud access is required")
	}
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

func getGCPConfig() (*compute.Service, []string, []int, string, string, string, error) {
	finalZones := map[string]int{}
	switch {
	case len(numNodes) != len(utils.Unique(cmdLineRegion)):
		return nil, nil, nil, "", "", "", errors.New("number of regions and number of nodes must be equal. Please make sure list of regions is unique")
	case len(cmdLineRegion) == 0 && len(numNodes) == 0:
		var err error
		finalZones, err = getRegionsNodeNum(constants.GCPCloudService)
		if err != nil {
			return nil, nil, nil, "", "", "", err
		}
	default:
		for i, region := range cmdLineRegion {
			finalZones[region] = numNodes[i]
		}
	}
	gcpClient, projectName, gcpCredentialFilePath, err := getGCPCloudCredentials()
	if err != nil {
		return nil, nil, nil, "", "", "", err
	}
	imageID, err := gcpAPI.GetUbuntuImageID(gcpClient)
	if err != nil {
		return nil, nil, nil, "", "", "", err
	}
	return gcpClient, maps.Keys(finalZones), maps.Values(finalZones), imageID, gcpCredentialFilePath, projectName, nil
}

func randomString(length int) string {
	rand.Seed(uint64(time.Now().UnixNano()))
	chars := "abcdefghijklmnopqrstuvwxyz"
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
	instanceType string,
	numNodes []int,
	zones []string,
	ami,
	cliDefaultName,
	projectName,
	credentialsPath string,
) (map[string][]string, map[string][]string, string, string, error) {
	keyPairName := fmt.Sprintf("%s-keypair", cliDefaultName)
	sshKeyPath, err := app.GetSSHCertFilePath(keyPairName)
	if err != nil {
		return nil, nil, "", "", err
	}
	networkName := fmt.Sprintf("%s-network", cliDefaultName)
	if err := terraformgcp.SetCloudCredentials(rootBody, zones, credentialsPath, projectName); err != nil {
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
		_, err = exec.Command("ssh-keygen", "-t", "rsa", "-f", sshKeyPath, "-C", "ubuntu", "-b", "2048").Output()
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
		terraformgcp.SetNetwork(rootBody, userIPAddress, networkName)
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("Using existing network %s in GCP", networkName))
		terraformgcp.SetExistingNetwork(rootBody, networkName)
		firewallName := fmt.Sprintf("%s-%s", networkName, strings.ReplaceAll(userIPAddress, ".", ""))
		firewallExists, err := gcpAPI.CheckFirewallExists(gcpClient, projectName, firewallName)
		if err != nil {
			return nil, nil, "", "", err
		}
		if !firewallExists {
			terraformgcp.SetFirewallRule(rootBody, userIPAddress+"/32", firewallName, networkName, []string{strconv.Itoa(constants.SSHTCPPort), strconv.Itoa(constants.AvalanchegoAPIPort)}, true)
		}
	}
	nodeName := map[string]string{}
	for _, zone := range zones {
		nodeName[zone] = randomString(5)
	}
	publicIPName := map[string]string{}
	if useStaticIP {
		for i, zone := range zones {
			publicIPName[zone] = fmt.Sprintf("static-ip-%s", nodeName[zone])
			terraformgcp.SetPublicIP(rootBody, zone, nodeName[zone], numNodes[i])
		}
	}
	sshPublicKey, err := os.ReadFile(fmt.Sprintf("%s.pub", sshKeyPath))
	if err != nil {
		return nil, nil, "", "", err
	}
	terraformgcp.SetupInstances(rootBody, zones, networkName, string(sshPublicKey), ami, publicIPName, nodeName, numNodes, instanceType, networkExists)
	if useStaticIP {
		terraformgcp.SetOutput(rootBody, zones)
	}
	err = app.CreateTerraformDir()
	if err != nil {
		return nil, nil, "", "", err
	}
	err = terraform.SaveConf(app.GetTerraformDir(), hclFile)
	if err != nil {
		return nil, nil, "", "", err
	}
	instanceIDs := map[string][]string{}
	for z, zone := range zones {
		instanceIDs[zone] = []string{}
		for i := 0; i < numNodes[z]; i++ {
			instanceIDs[zone] = append(instanceIDs[zone], fmt.Sprintf("%s-%s", nodeName[zone], strconv.Itoa(i)))
		}
	}

	elasticIPs, err := terraformgcp.RunTerraform(app.GetTerraformDir(), zones, useStaticIP)
	if err != nil {
		return instanceIDs, nil, "", "", errors.New(constants.ErrCreatingGCPNode)
	}
	ux.Logger.PrintToUser("New GCE instance(s) successfully created in Google Cloud Engine!")
	sshCertPath, err := app.GetSSHCertFilePath(fmt.Sprintf("%s-keypair", cliDefaultName))
	if err != nil {
		return nil, nil, "", "", err
	}
	return instanceIDs, elasticIPs, sshCertPath, keyPairName, nil
}

func createGCPInstance(
	usr *user.User,
	gcpClient *compute.Service,
	instanceType string,
	numNodes []int,
	zones []string,
	imageID string,
	gcpCredentialFilepath string,
	gcpProjectName string,
	clusterName string,
) (models.CloudConfig, error) {
	defaultAvalancheCLIPrefix := usr.Username + constants.AvalancheCLISuffix
	hclFile, rootBody, err := terraform.InitConf()
	if err != nil {
		return models.CloudConfig{}, err
	}
	instanceIDs, elasticIPs, certFilePath, keyPairName, err := createGCEInstances(
		rootBody,
		gcpClient,
		hclFile,
		instanceType,
		numNodes,
		zones,
		imageID,
		defaultAvalancheCLIPrefix,
		gcpProjectName,
		gcpCredentialFilepath,
	)
	if err != nil {
		ux.Logger.PrintToUser("Failed to create GCP cloud server")
		if err.Error() == constants.ErrCreatingGCPNode {
			// we stop created instances so that user doesn't pay for unused GCP instances
			ux.Logger.PrintToUser("Stopping all created GCP instances due to error to prevent charge for unused GCP instances...")
			failedNodes := map[string]error{}
			for zone, zoneInstances := range instanceIDs {
				for _, instanceID := range zoneInstances {
					nodeConfig := models.NodeConfig{
						NodeID: instanceID,
						Region: zone,
					}
					if stopErr := gcpAPI.StopGCPNode(gcpClient, nodeConfig, gcpProjectName, clusterName, false); err != nil {
						failedNodes[instanceID] = stopErr
						continue
					}
					ux.Logger.PrintToUser(fmt.Sprintf("GCP cloud server instance %s stopped in %s zone", instanceID, zone))
				}
			}
			if len(failedNodes) > 0 {
				ux.Logger.PrintToUser("Failed nodes: ")
				for node, err := range failedNodes {
					ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, err))
				}
				ux.Logger.PrintToUser("Stop the above instance(s) on GCP console to prevent charges")
				return models.CloudConfig{}, fmt.Errorf("failed to stop node(s) %s", failedNodes)
			}
		}
		return models.CloudConfig{}, err
	}
	ccm := models.CloudConfig{}
	for _, zone := range zones {
		ccm[zone] = models.RegionConfig{
			InstanceIDs:   instanceIDs[zone],
			PublicIPs:     elasticIPs[zone],
			KeyPair:       keyPairName,
			SecurityGroup: fmt.Sprintf("%s-network", defaultAvalancheCLIPrefix),
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
