// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	terraformgcp "github.com/ava-labs/avalanche-cli/pkg/terraform/gcp"
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
	ux.Logger.PrintToUser("To create a VM instance in GCP, we will need your service account credentials")
	ux.Logger.PrintToUser("Please follow instructions detailed at https://developers.google.com/workspace/guides/create-credentials#service-account to set up a GCP service account")
	ux.Logger.PrintToUser("Once completed, please enter the filepath to the JSON file containing the public/private key pair")
	ux.Logger.PrintToUser("For example: /Users/username/sample-project.json")
	return app.Prompt.CaptureString("What is the filepath to the credentials JSON file?")
}

func getGCPCloudCredentials() (*compute.Service, string, string, error) {
	var err error
	var gcpCredentialsPath string
	var gcpProjectName string
	clusterConfig := models.ClusterConfig{}
	if app.ClusterConfigExists() {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return nil, "", "", err
		}
		if clusterConfig.GCPConfig != nil {
			if _, ok := clusterConfig.GCPConfig[constants.GCPProjectNameClusterConfig]; ok {
				gcpProjectName = clusterConfig.GCPConfig[constants.GCPProjectNameClusterConfig]
			}
			if _, ok := clusterConfig.GCPConfig[constants.GCPServiceAccountFilePathClusterConfig]; ok {
				gcpCredentialsPath = clusterConfig.GCPConfig[constants.GCPServiceAccountFilePathClusterConfig]
			}
		}
	}
	if gcpProjectName == "" {
		gcpProjectName, err = app.Prompt.CaptureString("What is the name of your Google Cloud project?")
		if err != nil {
			return nil, "", "", err
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
	gcpClient, projectName, gcpCredentialFilePath, err := getGCPCloudCredentials()
	if err != nil {
		return nil, "", "", "", "", err
	}
	imageID, err := gcpAPI.GetUbuntuImageID(gcpClient)
	if err != nil {
		return nil, "", "", "", "", err
	}
	return gcpClient, zone, imageID, gcpCredentialFilePath, projectName, nil
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
	zone,
	ami,
	cliDefaultName,
	projectName,
	credentialsPath string,
) ([]string, []string, string, string, error) {
	keyPairName := fmt.Sprintf("%s-keypair", cliDefaultName)
	sshKeyPath, err := app.GetSSHCertFilePath(keyPairName)
	if err != nil {
		return nil, nil, "", "", err
	}
	networkName := fmt.Sprintf("%s-network", cliDefaultName)
	if err := terraformgcp.SetCloudCredentials(rootBody, zone, credentialsPath, projectName); err != nil {
		return nil, nil, "", "", err
	}
	numNodes, err := app.Prompt.CaptureUint32("How many nodes do you want to set up on GCP?")
	if err != nil {
		return nil, nil, "", "", err
	}
	if numNodes < math.MaxInt8 {
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
	nodeName := randomString(5)
	publicIPName := ""
	if useStaticIP {
		publicIPName = fmt.Sprintf("static-ip-%s", nodeName)
		terraformgcp.SetPublicIP(rootBody, nodeName, numNodes)
	}
	sshPublicKey, err := os.ReadFile(fmt.Sprintf("%s.pub", sshKeyPath))
	if err != nil {
		return nil, nil, "", "", err
	}
	terraformgcp.SetupInstances(rootBody, networkName, string(sshPublicKey), ami, publicIPName, nodeName, numNodes, networkExists)
	if useStaticIP {
		terraformgcp.SetOutput(rootBody)
	}
	err = app.CreateTerraformDir()
	if err != nil {
		return nil, nil, "", "", err
	}
	err = terraform.SaveConf(app.GetTerraformDir(), hclFile)
	if err != nil {
		return nil, nil, "", "", err
	}
	elasticIPs, err := terraformgcp.RunTerraform(app.GetTerraformDir(), useStaticIP)
	if err != nil {
		return nil, nil, "", "", err
	}
	instanceIDs := []string{}
	for i := 0; i < int(numNodes); i++ {
		instanceIDs = append(instanceIDs, fmt.Sprintf("%s-%s", nodeName, strconv.Itoa(i)))
	}
	ux.Logger.PrintToUser("New GCE instance(s) successfully created in Google Cloud Engine!")
	sshCertPath, err := app.GetSSHCertFilePath(fmt.Sprintf("%s-keypair", cliDefaultName))
	if err != nil {
		return nil, nil, "", "", err
	}
	return instanceIDs, elasticIPs, sshCertPath, keyPairName, nil
}

func createGCPInstance(usr *user.User, gcpClient *compute.Service, zone, imageID, gcpCredentialFilepath, gcpProjectName string) (CloudConfig, error) {
	defaultAvalancheCLIPrefix := usr.Username + constants.AvalancheCLISuffix
	hclFile, rootBody, err := terraform.InitConf()
	if err != nil {
		return CloudConfig{}, err
	}
	instanceIDs, elasticIPs, certFilePath, keyPairName, err := createGCEInstances(rootBody, gcpClient, hclFile, zone, imageID, defaultAvalancheCLIPrefix, gcpProjectName, gcpCredentialFilepath)
	if err != nil {
		ux.Logger.PrintToUser("Failed to create GCP cloud server")
		return CloudConfig{}, err
	}
	gcpCloudConfig := CloudConfig{
		instanceIDs,
		elasticIPs,
		zone,
		keyPairName,
		fmt.Sprintf("%s-network", defaultAvalancheCLIPrefix),
		certFilePath,
		imageID,
	}
	return gcpCloudConfig, nil
}

func updateClusterConfigGCPKeyFilepath(projectName, serviceAccountKeyFilepath string) error {
	clusterConfig := models.ClusterConfig{}
	var err error
	if app.ClusterConfigExists() {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return err
		}
	}
	if clusterConfig.GCPConfig == nil {
		clusterConfig.GCPConfig = make(map[string]string)
	}
	if projectName != "" {
		clusterConfig.GCPConfig[constants.GCPProjectNameClusterConfig] = projectName
	}
	if serviceAccountKeyFilepath != "" {
		clusterConfig.GCPConfig[constants.GCPServiceAccountFilePathClusterConfig] = serviceAccountKeyFilepath
	}
	return app.WriteClusterConfigFile(&clusterConfig)
}
