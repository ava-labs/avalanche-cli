// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os/exec"
	"os/user"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func getNewKeyPairName(ec2Svc *awsAPI.AwsCloud) (string, error) {
	newKeyPairName := cmdLineAlternativeKeyPairName
	for {
		if newKeyPairName != "" {
			keyPairExists, err := ec2Svc.CheckKeyPairExists(newKeyPairName)
			if err != nil {
				return "", err
			}
			if !keyPairExists {
				return newKeyPairName, nil
			}
			ux.Logger.PrintToUser(fmt.Sprintf("Key Pair named %s already exists", newKeyPairName))
		}
		ux.Logger.PrintToUser("What do you want to name your key pair?")
		var err error
		newKeyPairName, err = app.Prompt.CaptureString("Key Pair Name")
		if err != nil {
			return "", err
		}
	}
}

func printNoCredentialsOutput(awsProfile string) {
	ux.Logger.PrintToUser("No AWS credentials found in file ~/.aws/credentials ")
	ux.Logger.PrintToUser("Or in environment variables AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
	ux.Logger.PrintToUser("Please make sure correspoding keys are set in [%s] section in ~/.aws/credentials", awsProfile)
	ux.Logger.PrintToUser("Or create a file called 'credentials' with the contents below, and add the file to ~/.aws/ directory if it's not already there")
	ux.Logger.PrintToUser("===========BEGINNING OF FILE===========")
	ux.Logger.PrintToUser("[%s]\naws_access_key_id=<AWS_ACCESS_KEY>\naws_secret_access_key=<AWS_SECRET_ACCESS_KEY>", awsProfile)
	ux.Logger.PrintToUser("===========END OF FILE===========")
	ux.Logger.PrintToUser("More info can be found at https://docs.aws.amazon.com/sdkref/latest/guide/file-format.html#file-format-creds")
	ux.Logger.PrintToUser("Also you can set environment variables AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
	ux.Logger.PrintToUser("Please use https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html#envvars-set for more details")
}

func isExpiredCredentialError(err error) bool {
	return strings.Contains(err.Error(), "RequestExpired: Request has expired")
}

func printExpiredCredentialsOutput(awsProfile string) {
	ux.Logger.PrintToUser("AWS credentials expired")
	ux.Logger.PrintToUser("Please update your environment variables AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
	ux.Logger.PrintToUser("Following https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html#envvars-set")
	ux.Logger.PrintToUser("Or fill in ~/.aws/credentials with updated contents following the format below")
	ux.Logger.PrintToUser("===========BEGINNING OF FILE===========")
	ux.Logger.PrintToUser("[%s]\naws_access_key_id=<AWS_ACCESS_KEY>\naws_secret_access_key=<AWS_SECRET_ACCESS_KEY>", awsProfile)
	ux.Logger.PrintToUser("===========END OF FILE===========")
	ux.Logger.PrintToUser("More info can be found at https://docs.aws.amazon.com/sdkref/latest/guide/file-format.html#file-format-creds")
	ux.Logger.PrintToUser("")
}

// getAWSCloudCredentials gets AWS account credentials defined in .aws dir in user home dir
func getAWSCloudCredentials(awsProfile, region string) (*awsAPI.AwsCloud, error) {
	return awsAPI.NewAwsCloud(awsProfile, region)
}

// promptKeyPairName get custom name for key pair if the default key pair name that we use cannot be used for this EC2 instance
func promptKeyPairName(ec2 *awsAPI.AwsCloud) (string, error) {
	newKeyPairName, err := getNewKeyPairName(ec2)
	if err != nil {
		return "", err
	}
	return newKeyPairName, nil
}

func getAWSMonitoringEC2Svc(awsProfile, monitoringRegion string) (map[string]*awsAPI.AwsCloud, error) {
	ec2SvcMap := map[string]*awsAPI.AwsCloud{}
	var err error
	ec2SvcMap[monitoringRegion], err = getAWSCloudCredentials(awsProfile, monitoringRegion)
	if err != nil {
		if !strings.Contains(err.Error(), "cloud access is required") {
			printNoCredentialsOutput(awsProfile)
		}
		return nil, err
	}
	return ec2SvcMap, nil
}

func getAWSCloudConfig(awsProfile string) (map[string]*awsAPI.AwsCloud, map[string]string, map[string]int, error) {
	finalRegions := map[string]int{}
	switch {
	case len(numNodes) != len(utils.Unique(cmdLineRegion)):
		return nil, nil, nil, fmt.Errorf("number of nodes and regions should be the same")
	case len(cmdLineRegion) == 0 && len(numNodes) == 0:
		var err error
		finalRegions, err = getRegionsNodeNum(constants.AWSCloudService)
		if err != nil {
			return nil, nil, nil, err
		}
	default:
		for i, region := range cmdLineRegion {
			finalRegions[region] = numNodes[i]
		}
	}
	ec2SvcMap := map[string]*awsAPI.AwsCloud{}
	amiMap := map[string]string{}
	numNodesMap := map[string]int{}
	for region := range finalRegions {
		var err error
		ec2SvcMap[region], err = getAWSCloudCredentials(awsProfile, region)
		if err != nil {
			if !strings.Contains(err.Error(), "cloud access is required") {
				printNoCredentialsOutput(awsProfile)
			}
			return nil, nil, nil, err
		}
		amiMap[region], err = ec2SvcMap[region].GetUbuntuAMIID()
		if err != nil {
			if isExpiredCredentialError(err) {
				printExpiredCredentialsOutput(awsProfile)
			}
			return nil, nil, nil, err
		}
		numNodesMap[region] = finalRegions[region]
	}
	return ec2SvcMap, amiMap, numNodesMap, nil
}

// createEC2Instances creates  ec2 instances
func createEC2Instances(ec2Svc map[string]*awsAPI.AwsCloud,
	regions []string,
	regionConf map[string]models.RegionConfig,
	forMonitoring bool,
) (map[string][]string, map[string][]string, map[string]string, map[string]string, error) {
	if !forMonitoring {
		ux.Logger.PrintToUser("Creating new EC2 instance(s) on AWS...")
	} else {
		ux.Logger.PrintToUser("Creating separate monitoring EC2 instance(s) on AWS...")
	}

	userIPAddress, err := getIPAddress()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	useExistingKeyPair := map[string]bool{}
	keyPairName := map[string]string{}
	instanceIDs := map[string][]string{}
	elasticIPs := map[string][]string{}
	sshCertPath := map[string]string{}
	for _, region := range regions {
		keyPairExists, err := ec2Svc[region].CheckKeyPairExists(regionConf[region].Prefix)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		certInSSHDir, err := app.CheckCertInSSHDir(regionConf[region].CertName)
		if useSSHAgent {
			certInSSHDir = true // if using ssh agent, we consider that we have a cert on hand
		}
		if err != nil {
			return nil, nil, nil, nil, err
		}
		sgID := ""
		keyPairName[region] = regionConf[region].Prefix
		securityGroupName := regionConf[region].SecurityGroupName
		privKey, err := app.GetSSHCertFilePath(regionConf[region].CertName)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if !keyPairExists {
			switch {
			case useSSHAgent:
				ux.Logger.PrintToUser("Using ssh agent identity %s to create key pair %s in AWS[%s]", sshIdentity, keyPairName[region], region)
				if err := ec2Svc[region].UploadSSHIdentityKeyPair(regionConf[region].Prefix, sshIdentity); err != nil {
					return nil, nil, nil, nil, err
				}
			case !useSSHAgent && certInSSHDir:
				ux.Logger.PrintToUser("Default Key Pair named %s already exists on your .ssh directory but not on AWS", regionConf[region].Prefix)
				ux.Logger.PrintToUser("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in AWS[%s]", regionConf[region].Prefix, region)
				keyPairName[region], err = promptKeyPairName(ec2Svc[region])
				if err != nil {
					return nil, nil, nil, nil, err
				}
				if err := ec2Svc[region].CreateAndDownloadKeyPair(regionConf[region].Prefix, privKey); err != nil {
					return nil, nil, nil, nil, err
				}
			case !useSSHAgent && !certInSSHDir:
				ux.Logger.PrintToUser(fmt.Sprintf("Creating new key pair %s in AWS[%s]", keyPairName, region))
				if err := ec2Svc[region].CreateAndDownloadKeyPair(regionConf[region].Prefix, privKey); err != nil {
					return nil, nil, nil, nil, err
				}
			}
		} else {
			// keypair exists
			switch {
			case useSSHAgent:
				ux.Logger.PrintToUser("Using existing key pair %s in AWS[%s] via ssh-agent", keyPairName[region], region)
				useExistingKeyPair[region] = true
			case !useSSHAgent && certInSSHDir:
				ux.Logger.PrintToUser("Using existing key pair %s in AWS[%s]", keyPairName[region], region)
				useExistingKeyPair[region] = true
			case !useSSHAgent && !certInSSHDir:
				ux.Logger.PrintToUser("Default Key Pair named %s already exists in AWS[%s]", keyPairName[region], region)
				ux.Logger.PrintToUser("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in your .ssh directory", keyPairName)
				keyPairName[region], err = promptKeyPairName(ec2Svc[region])
				if err != nil {
					return nil, nil, nil, nil, err
				}
				if err := ec2Svc[region].CreateAndDownloadKeyPair(regionConf[region].Prefix, privKey); err != nil {
					return nil, nil, nil, nil, err
				}
			}
		}
		securityGroupExists, sg, err := ec2Svc[region].CheckSecurityGroupExists(regionConf[region].SecurityGroupName)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if !securityGroupExists {
			ux.Logger.PrintToUser(fmt.Sprintf("Creating new security group %s in AWS[%s]", securityGroupName, region))
			if newSGID, err := ec2Svc[region].SetupSecurityGroup(userIPAddress, regionConf[region].SecurityGroupName); err != nil {
				return nil, nil, nil, nil, err
			} else {
				sgID = newSGID
			}
		} else {
			sgID = *sg.GroupId
			ux.Logger.PrintToUser(fmt.Sprintf("Using existing security group %s in AWS[%s]", securityGroupName, region))
			ipInTCP := awsAPI.CheckUserIPInSg(&sg, userIPAddress, constants.SSHTCPPort)
			ipInHTTP := awsAPI.CheckUserIPInSg(&sg, userIPAddress, constants.AvalanchegoAPIPort)
			ipInMonitoring := awsAPI.CheckUserIPInSg(&sg, userIPAddress, constants.AvalanchegoMonitoringPort)
			ipInGrafana := awsAPI.CheckUserIPInSg(&sg, userIPAddress, constants.AvalanchegoGrafanaPort)

			if !ipInTCP {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", userIPAddress, constants.SSHTCPPort); err != nil {
					return nil, nil, nil, nil, err
				}
			}
			if !ipInHTTP {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", userIPAddress, constants.AvalanchegoAPIPort); err != nil {
					return nil, nil, nil, nil, err
				}
			}
			if !ipInMonitoring {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", userIPAddress, constants.AvalanchegoMonitoringPort); err != nil {
					return nil, nil, nil, nil, err
				}
			}
			if !ipInGrafana {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", userIPAddress, constants.AvalanchegoGrafanaPort); err != nil {
					return nil, nil, nil, nil, err
				}
			}
		}
		sshCertPath[region] = privKey
		if instanceIDs[region], err = ec2Svc[region].CreateEC2Instances(
			regionConf[region].NumNodes,
			regionConf[region].ImageID,
			regionConf[region].InstanceType,
			keyPairName[region],
			sgID,
			forMonitoring,
		); err != nil {
			return nil, nil, nil, nil, err
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Waiting for EC2 instances in AWS[%s] to be provisioned...", region))
		if err := ec2Svc[region].WaitForEC2Instances(instanceIDs[region]); err != nil {
			return nil, nil, nil, nil, err
		}
		if useStaticIP {
			publicIPs := []string{}
			for count := 0; count < regionConf[region].NumNodes; count++ {
				allocationID, publicIP, err := ec2Svc[region].CreateEIP()
				if err != nil {
					return nil, nil, nil, nil, err
				}
				if err := ec2Svc[region].AssociateEIP(instanceIDs[region][count], allocationID); err != nil {
					return nil, nil, nil, nil, err
				}
				publicIPs = append(publicIPs, publicIP)
			}
			elasticIPs[region] = publicIPs
		} else {
			instanceEIPMap, err := ec2Svc[region].GetInstancePublicIPs(instanceIDs[region])
			if err != nil {
				return nil, nil, nil, nil, err
			}
			regionElasticIPs := []string{}
			for _, instanceID := range instanceIDs[region] {
				regionElasticIPs = append(regionElasticIPs, instanceEIPMap[instanceID])
			}
			elasticIPs[region] = regionElasticIPs
		}
	}
	ux.Logger.PrintToUser("New EC2 instance(s) successfully created in AWS!")
	for _, region := range regions {
		if !useExistingKeyPair[region] && !useSSHAgent {
			// takes the cert file downloaded from AWS and moves it to .ssh directory
			err = addCertToSSH(regionConf[region].CertName)
			if err != nil {
				return nil, nil, nil, nil, err
			}
		}
		if useSSHAgent {
			sshCertPath[region] = ""
		} else {
			sshCertPath[region], err = app.GetSSHCertFilePath(regionConf[region].CertName)
			if err != nil {
				return nil, nil, nil, nil, err
			}
		}
	}
	// instanceIDs, elasticIPs, certFilePath, keyPairName, err
	return instanceIDs, elasticIPs, sshCertPath, keyPairName, nil
}

func AddMonitoringSecurityGroupRule(ec2Svc map[string]*awsAPI.AwsCloud, monitoringHostPublicIP, securityGroupName, region string) error {
	securityGroupExists, sg, err := ec2Svc[region].CheckSecurityGroupExists(securityGroupName)
	if err != nil {
		return err
	}
	if !securityGroupExists {
		return fmt.Errorf("security group %s doesn't exist in region %s", securityGroupName, region)
	}
	metricsPortInSG := awsAPI.CheckUserIPInSg(&sg, monitoringHostPublicIP, constants.AvalanchegoMachineMetricsPort)
	apiPortInSG := awsAPI.CheckUserIPInSg(&sg, monitoringHostPublicIP, constants.AvalanchegoAPIPort)
	if !metricsPortInSG {
		if err = ec2Svc[region].AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", monitoringHostPublicIP+constants.IPAddressSuffix, constants.AvalanchegoMachineMetricsPort); err != nil {
			return err
		}
	}
	if !apiPortInSG {
		if err = ec2Svc[region].AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", monitoringHostPublicIP+constants.IPAddressSuffix, constants.AvalanchegoAPIPort); err != nil {
			return err
		}
	}
	return nil
}

func createAWSInstances(
	ec2Svc map[string]*awsAPI.AwsCloud,
	nodeType string,
	numNodes map[string]int,
	regions []string,
	ami map[string]string,
	usr *user.User,
	forMonitoring bool) (
	models.CloudConfig, error,
) {
	regionConf := map[string]models.RegionConfig{}
	for _, region := range regions {
		prefix := usr.Username + "-" + region + constants.AvalancheCLISuffix
		regionConf[region] = models.RegionConfig{
			Prefix:            prefix,
			ImageID:           ami[region],
			CertName:          prefix + "-" + region + constants.CertSuffix,
			SecurityGroupName: prefix + "-" + region + constants.AWSSecurityGroupSuffix,
			NumNodes:          numNodes[region],
			InstanceType:      nodeType,
		}
	}
	// Create new EC2 instances
	instanceIDs, elasticIPs, certFilePath, keyPairName, err := createEC2Instances(ec2Svc, regions, regionConf, forMonitoring)
	if err != nil {
		if err.Error() == constants.EIPLimitErr {
			ux.Logger.PrintToUser("Failed to create AWS cloud server(s), please try creating again in a different region")
		} else {
			ux.Logger.PrintToUser("Failed to create AWS cloud server(s) with error: %s", err.Error())
		}
		// we stop created instances so that user doesn't pay for unused EC2 instances
		ux.Logger.PrintToUser("Stopping all created AWS instances due to error to prevent charge for unused AWS instances...")
		failedNodes := map[string]error{}
		for region, regionInstanceID := range instanceIDs {
			for _, instanceID := range regionInstanceID {
				ux.Logger.PrintToUser(fmt.Sprintf("Stopping AWS cloud server %s...", instanceID))
				if stopErr := ec2Svc[region].StopInstance(instanceID, "", true); stopErr != nil {
					failedNodes[instanceID] = stopErr
				}
				ux.Logger.PrintToUser(fmt.Sprintf("AWS cloud server instance %s stopped", instanceID))
			}
		}
		if len(failedNodes) > 0 {
			ux.Logger.PrintToUser("Failed nodes: ")
			for node, err := range failedNodes {
				ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, err))
			}
			ux.Logger.PrintToUser("Stop the above instance(s) on AWS console to prevent charges")
			return models.CloudConfig{}, fmt.Errorf("failed to stop node(s) %s", failedNodes)
		}
		return models.CloudConfig{}, err
	}
	awsCloudConfig := models.CloudConfig{}
	for _, region := range regions {
		awsCloudConfig[region] = models.RegionConfig{
			InstanceIDs:   instanceIDs[region],
			PublicIPs:     elasticIPs[region],
			KeyPair:       keyPairName[region],
			SecurityGroup: regionConf[region].SecurityGroupName,
			CertFilePath:  certFilePath[region],
			ImageID:       ami[region],
		}
	}
	return awsCloudConfig, nil
}

// addCertToSSH takes the cert file downloaded from AWS and moves it to .ssh directory
func addCertToSSH(certName string) error {
	certFilePath, err := app.GetSSHCertFilePath(certName)
	if err != nil {
		return err
	}
	cmd := exec.Command("ssh-add", certFilePath)
	utils.SetupRealtimeCLIOutput(cmd, true, true)
	return cmd.Run()
}
