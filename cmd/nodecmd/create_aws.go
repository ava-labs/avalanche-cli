// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"golang.org/x/exp/slices"

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

func getAWSCloudConfig(awsProfile string, singleNode bool, clusterSgRegions []string, instanceType string) (map[string]*awsAPI.AwsCloud, map[string]string, map[string]NumNodes, error) {
	finalRegions := map[string]NumNodes{}
	switch {
	case len(numValidatorsNodes) != len(utils.Unique(cmdLineRegion)):
		return nil, nil, nil, fmt.Errorf("number of nodes and regions should be the same")
	case (globalNetworkFlags.UseDevnet || globalNetworkFlags.UseFuji) && len(numAPINodes) != 0 && len(numAPINodes) != len(utils.Unique(cmdLineRegion)):
		return nil, nil, nil, fmt.Errorf("number of api nodes and regions should be the same")
	case (globalNetworkFlags.UseDevnet || globalNetworkFlags.UseFuji) && len(numAPINodes) != 0 && len(numAPINodes) != len(numValidatorsNodes):
		return nil, nil, nil, fmt.Errorf("number of api nodes and validator nodes should be the same")
	case len(cmdLineRegion) == 0 && len(numValidatorsNodes) == 0 && len(numAPINodes) == 0:
		var err error
		if singleNode {
			selectedRegion := ""
			if loadTestHostRegion != "" {
				selectedRegion = loadTestHostRegion
			} else {
				selectedRegion, err = getSeparateHostNodeParam(constants.AWSCloudService)
				if err != nil {
					return nil, nil, nil, err
				}
			}
			finalRegions = map[string]NumNodes{selectedRegion: {1, 0}}
		} else {
			finalRegions, err = getRegionsNodeNum(constants.AWSCloudService)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	default:
		for i, region := range cmdLineRegion {
			numAPINodesInRegion := 0
			if len(numAPINodes) > 0 {
				numAPINodesInRegion = numAPINodes[i]
			}
			if globalNetworkFlags.UseDevnet || globalNetworkFlags.UseFuji {
				finalRegions[region] = NumNodes{numValidatorsNodes[i], numAPINodesInRegion}
			} else {
				finalRegions[region] = NumNodes{numValidatorsNodes[i], 0}
			}
		}
	}
	ec2SvcMap := map[string]*awsAPI.AwsCloud{}
	amiMap := map[string]string{}
	numNodesMap := map[string]NumNodes{}
	// verify regions are valid
	if invalidRegions, err := checkRegions(maps.Keys(finalRegions)); err != nil {
		return nil, nil, nil, err
	} else if len(invalidRegions) > 0 {
		return nil, nil, nil, fmt.Errorf("invalid regions %s provided for %s", invalidRegions, constants.AWSCloudService)
	}
	for region := range finalRegions {
		var err error
		if singleNode {
			for _, clusterRegion := range clusterSgRegions {
				ec2SvcMap[clusterRegion], err = getAWSCloudCredentials(awsProfile, clusterRegion)
				if err != nil {
					if !strings.Contains(err.Error(), "cloud access is required") {
						printNoCredentialsOutput(awsProfile)
					}
					return nil, nil, nil, err
				}
			}
		} else {
			ec2SvcMap[region], err = getAWSCloudCredentials(awsProfile, region)
			if err != nil {
				if !strings.Contains(err.Error(), "cloud access is required") {
					printNoCredentialsOutput(awsProfile)
				}
				return nil, nil, nil, err
			}
		}
		arch, err := ec2SvcMap[region].GetInstanceTypeArch(instanceType)
		if err != nil {
			return nil, nil, nil, err
		}
		amiMap[region], err = ec2SvcMap[region].GetUbuntuAMIID(arch, constants.UbuntuVersionLTS)
		if err != nil {
			if isExpiredCredentialError(err) {
				printExpiredCredentialsOutput(awsProfile)
			}
			return nil, nil, nil, err
		}
		isSupported, err := ec2SvcMap[region].IsInstanceTypeSupported(instanceType)
		if err != nil {
			return nil, nil, nil, err
		} else if !isSupported {
			return nil, nil, nil, fmt.Errorf("instance type %s is not supported in region %s", instanceType, region)
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
	publicHTTPPortAccess bool,
) (map[string][]string, map[string][]string, map[string]string, map[string]string, error) {
	if !forMonitoring {
		ux.Logger.PrintToUser("Creating new EC2 instance(s) on AWS...")
	} else {
		ux.Logger.PrintToUser("Creating separate monitoring EC2 instance(s) on AWS...")
	}
	userIPAddress, err := utils.GetUserIPAddress()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	keyPairName := map[string]string{}
	instanceIDs := map[string][]string{}
	elasticIPs := map[string][]string{}
	sshCertPath := map[string]string{}
	for _, region := range regions {
		keyPairExists, err := ec2Svc[region].CheckKeyPairExists(regionConf[region].Prefix)
		if err != nil {
			return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
		}
		certInSSHDir, err := app.CheckCertInSSHDir(regionConf[region].CertName)
		if useSSHAgent {
			certInSSHDir = true // if using ssh agent, we consider that we have a cert on hand
		}
		if err != nil {
			return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
		}
		sgID := ""
		keyPairName[region] = regionConf[region].Prefix
		securityGroupName := regionConf[region].SecurityGroupName
		privKey, err := app.GetSSHCertFilePath(regionConf[region].CertName)
		if err != nil {
			return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
		}
		if replaceKeyPair && !forMonitoring {
			// delete existing key pair on AWS console and download the newly created key pair file
			// in .ssh dir (will overwrite existing file in .ssh dir)
			if keyPairExists {
				if err := ec2Svc[region].DeleteKeyPair(regionConf[region].Prefix); err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, fmt.Errorf("unable to delete existing key pair %s in AWS console due to %w", regionConf[region].Prefix, err)
				}
			}
			if err = os.RemoveAll(privKey); err != nil {
				return instanceIDs, elasticIPs, sshCertPath, keyPairName, fmt.Errorf("unable to delete existing key pair file %s in .ssh dir due to %w", privKey, err)
			}
			if err := ec2Svc[region].CreateAndDownloadKeyPair(regionConf[region].Prefix, privKey); err != nil {
				return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
			}
		} else {
			if !keyPairExists {
				switch {
				case useSSHAgent:
					ux.Logger.PrintToUser("Using ssh agent identity %s to create key pair %s in AWS[%s]", sshIdentity, keyPairName[region], region)
					if err := ec2Svc[region].UploadSSHIdentityKeyPair(regionConf[region].Prefix, sshIdentity); err != nil {
						return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
					}
				case !useSSHAgent && certInSSHDir:
					ux.Logger.PrintToUser("Default Key Pair named %s already exists on your .ssh directory but not on AWS", regionConf[region].Prefix)
					ux.Logger.PrintToUser("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in AWS[%s]", regionConf[region].Prefix, region)
					keyPairName[region], err = promptKeyPairName(ec2Svc[region])
					if err != nil {
						return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
					}
					if err := ec2Svc[region].CreateAndDownloadKeyPair(regionConf[region].Prefix, privKey); err != nil {
						return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
					}
				case !useSSHAgent && !certInSSHDir:
					ux.Logger.PrintToUser(fmt.Sprintf("Creating new key pair %s in AWS[%s]", keyPairName, region))
					if err := ec2Svc[region].CreateAndDownloadKeyPair(regionConf[region].Prefix, privKey); err != nil {
						return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
					}
				}
			} else {
				// keypair exists
				switch {
				case useSSHAgent:
					ux.Logger.PrintToUser("Using existing key pair %s in AWS[%s] via ssh-agent", keyPairName[region], region)
				case !useSSHAgent && certInSSHDir:
					ux.Logger.PrintToUser("Using existing key pair %s in AWS[%s]", keyPairName[region], region)
				case !useSSHAgent && !certInSSHDir:
					ux.Logger.PrintToUser("Default Key Pair named %s already exists in AWS[%s]", keyPairName[region], region)
					ux.Logger.PrintToUser("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in your .ssh directory", keyPairName[region])
					keyPairName[region], err = promptKeyPairName(ec2Svc[region])
					if err != nil {
						return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
					}
					privKey, err = app.GetSSHCertFilePath(keyPairName[region] + constants.CertSuffix)
					if err != nil {
						return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
					}
					if err := ec2Svc[region].CreateAndDownloadKeyPair(keyPairName[region], privKey); err != nil {
						return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
					}
				}
			}
		}
		securityGroupExists, sg, err := ec2Svc[region].CheckSecurityGroupExists(regionConf[region].SecurityGroupName)
		if err != nil {
			return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
		}
		if !securityGroupExists {
			ux.Logger.PrintToUser(fmt.Sprintf("Creating new security group %s in AWS[%s]", securityGroupName, region))
			if newSGID, err := ec2Svc[region].SetupSecurityGroup(userIPAddress, regionConf[region].SecurityGroupName); err != nil {
				return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
			} else {
				sgID = newSGID
			}
			// allow public access to API avalanchego port
			if publicHTTPPortAccess {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", "0.0.0.0/0", constants.AvalancheGoAPIPort); err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
			}
		} else {
			sgID = *sg.GroupId
			ux.Logger.PrintToUser(fmt.Sprintf("Using existing security group %s in AWS[%s]", securityGroupName, region))
			ipInTCP := awsAPI.CheckIPInSg(&sg, userIPAddress, constants.SSHTCPPort)
			ipInHTTP := awsAPI.CheckIPInSg(&sg, userIPAddress, constants.AvalancheGoAPIPort)
			ipInMonitoring := awsAPI.CheckIPInSg(&sg, userIPAddress, constants.AvalancheGoMonitoringPort)
			ipInGrafana := awsAPI.CheckIPInSg(&sg, userIPAddress, constants.AvalancheGoGrafanaPort)
			ipInLoki := awsAPI.CheckIPInSg(&sg, "0.0.0.0/0", constants.AvalancheGoLokiPort)

			if !ipInTCP {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", userIPAddress, constants.SSHTCPPort); err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
			}
			if !ipInHTTP {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", userIPAddress, constants.AvalancheGoAPIPort); err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
			}
			if !ipInMonitoring {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", userIPAddress, constants.AvalancheGoMonitoringPort); err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
			}
			if !ipInGrafana {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", userIPAddress, constants.AvalancheGoGrafanaPort); err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
			}
			if !ipInLoki {
				if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", "0.0.0.0/0", constants.AvalancheGoLokiPort); err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
			}
			// check for public access to API port if flag is set
			if publicHTTPPortAccess {
				ipInPublicAPI := awsAPI.CheckIPInSg(&sg, "0.0.0.0/0", constants.AvalancheGoAPIPort)
				if !ipInPublicAPI {
					if err := ec2Svc[region].AddSecurityGroupRule(sgID, "ingress", "tcp", "0.0.0.0/0", constants.AvalancheGoAPIPort); err != nil {
						return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
					}
				}
			}
		}
		sshCertPath[region] = privKey
		if instanceIDs[region], err = ec2Svc[region].CreateEC2Instances(
			regionConf[region].Prefix,
			regionConf[region].NumNodes,
			regionConf[region].ImageID,
			regionConf[region].InstanceType,
			keyPairName[region],
			sgID,
			forMonitoring,
			iops,
			throughput,
			stringToAWSVolumeType(volumeType),
			volumeSize,
		); err != nil {
			return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
		}
		spinSession := ux.NewUserSpinner()
		spinner := spinSession.SpinToUser("Waiting for EC2 instance(s) in AWS[%s] to be provisioned...", region)
		if err := ec2Svc[region].WaitForEC2Instances(instanceIDs[region], types.InstanceStateNameRunning); err != nil {
			ux.SpinFailWithError(spinner, "", err)
			return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
		}
		ux.SpinComplete(spinner)
		spinSession.Stop()
		if useStaticIP {
			publicIPs := []string{}
			for count := 0; count < regionConf[region].NumNodes; count++ {
				allocationID, publicIP, err := ec2Svc[region].CreateEIP(regionConf[region].Prefix)
				if err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
				if err := ec2Svc[region].AssociateEIP(instanceIDs[region][count], allocationID); err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
				publicIPs = append(publicIPs, publicIP)
			}
			elasticIPs[region] = publicIPs
		} else {
			instanceEIPMap, err := ec2Svc[region].GetInstancePublicIPs(instanceIDs[region])
			if err != nil {
				return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
			}
			regionElasticIPs := []string{}
			for _, instanceID := range instanceIDs[region] {
				regionElasticIPs = append(regionElasticIPs, instanceEIPMap[instanceID])
			}
			elasticIPs[region] = regionElasticIPs
		}
	}
	ux.Logger.GreenCheckmarkToUser("New EC2 instance(s) successfully created in AWS!")
	for _, region := range regions {
		if useSSHAgent {
			sshCertPath[region] = ""
		} else {
			// don't overwrite existing sshCertPath for a particular region
			if _, ok := sshCertPath[region]; !ok {
				sshCertPath[region], err = app.GetSSHCertFilePath(regionConf[region].CertName)
				if err != nil {
					return instanceIDs, elasticIPs, sshCertPath, keyPairName, err
				}
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
	metricsPortInSG := awsAPI.CheckIPInSg(&sg, monitoringHostPublicIP, constants.AvalancheGoMachineMetricsPort)
	apiPortInSG := awsAPI.CheckIPInSg(&sg, monitoringHostPublicIP, constants.AvalancheGoAPIPort)
	if !metricsPortInSG {
		if err = ec2Svc[region].AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", monitoringHostPublicIP+constants.IPAddressSuffix, constants.AvalancheGoMachineMetricsPort); err != nil {
			return err
		}
	}
	if !apiPortInSG {
		if err = ec2Svc[region].AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", monitoringHostPublicIP+constants.IPAddressSuffix, constants.AvalancheGoAPIPort); err != nil {
			return err
		}
	}
	return nil
}

func deleteHostSecurityGroupRule(ec2Svc *awsAPI.AwsCloud, hostPublicIP, securityGroupName string) error {
	securityGroupExists, sg, err := ec2Svc.CheckSecurityGroupExists(securityGroupName)
	if err != nil {
		return err
	}
	// exit early if security group doesn't exist
	if !securityGroupExists {
		return nil
	}
	metricsPortInSG := awsAPI.CheckIPInSg(&sg, hostPublicIP, constants.AvalancheGoMachineMetricsPort)
	apiPortInSG := awsAPI.CheckIPInSg(&sg, hostPublicIP, constants.AvalancheGoAPIPort)
	if metricsPortInSG {
		if err = ec2Svc.DeleteSecurityGroupRule(*sg.GroupId, "ingress", "tcp", hostPublicIP+constants.IPAddressSuffix, constants.AvalancheGoMachineMetricsPort); err != nil {
			return err
		}
	}
	if apiPortInSG {
		if err = ec2Svc.DeleteSecurityGroupRule(*sg.GroupId, "ingress", "tcp", hostPublicIP+constants.IPAddressSuffix, constants.AvalancheGoAPIPort); err != nil {
			return err
		}
	}
	return nil
}

func grantAccessToPublicIPViaSecurityGroup(ec2Svc *awsAPI.AwsCloud, publicIP, securityGroupName, region string) error {
	securityGroupExists, sg, err := ec2Svc.CheckSecurityGroupExists(securityGroupName)
	if err != nil {
		return err
	}
	if !securityGroupExists {
		return fmt.Errorf("security group %s doesn't exist in region %s", securityGroupName, region)
	}
	metricsPortInSG := awsAPI.CheckIPInSg(&sg, publicIP, constants.AvalancheGoMachineMetricsPort)
	apiPortInSG := awsAPI.CheckIPInSg(&sg, publicIP, constants.AvalancheGoAPIPort)
	if !metricsPortInSG {
		if err = ec2Svc.AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", publicIP+constants.IPAddressSuffix, constants.AvalancheGoMachineMetricsPort); err != nil {
			return err
		}
	}
	if !apiPortInSG {
		if err = ec2Svc.AddSecurityGroupRule(*sg.GroupId, "ingress", "tcp", publicIP+constants.IPAddressSuffix, constants.AvalancheGoAPIPort); err != nil {
			return err
		}
	}
	return nil
}

func createAWSInstances(
	ec2Svc map[string]*awsAPI.AwsCloud,
	nodeType string,
	numNodes map[string]NumNodes,
	regions []string,
	ami map[string]string,
	forMonitoring bool,
	publicHTTPPortAccess bool) (
	models.CloudConfig, error,
) {
	regionConf := map[string]models.RegionConfig{}
	for _, region := range regions {
		prefix, err := defaultAvalancheCLIPrefix(region)
		if err != nil {
			return models.CloudConfig{}, err
		}
		regionConf[region] = models.RegionConfig{
			Prefix:            prefix,
			ImageID:           ami[region],
			CertName:          prefix + "-" + region + constants.CertSuffix,
			SecurityGroupName: prefix + "-" + region + constants.AWSSecurityGroupSuffix,
			NumNodes:          numNodes[region].All(),
			InstanceType:      nodeType,
		}
	}
	// Create new EC2 instances
	instanceIDs, elasticIPs, certFilePath, keyPairName, err := createEC2Instances(ec2Svc, regions, regionConf, forMonitoring, publicHTTPPortAccess)
	if err != nil {
		if err.Error() == constants.EIPLimitErr {
			ux.Logger.PrintToUser("Failed to create AWS cloud server(s), please try creating again in a different region")
		} else {
			ux.Logger.PrintToUser("Failed to create AWS cloud server(s) with error: %s", err.Error())
		}
		// we destroy created instances so that user doesn't pay for unused EC2 instances
		ux.Logger.PrintToUser("Destroying all created AWS instances due to error to prevent charge for unused AWS instances...")
		failedNodes := map[string]error{}
		for region, regionInstanceID := range instanceIDs {
			for _, instanceID := range regionInstanceID {
				ux.Logger.PrintToUser(fmt.Sprintf("Destroying AWS cloud server %s...", instanceID))
				if destroyErr := ec2Svc[region].DestroyInstance(instanceID, "", true); destroyErr != nil {
					failedNodes[instanceID] = destroyErr
				}
				ux.Logger.PrintToUser(fmt.Sprintf("AWS cloud server instance %s destroyed", instanceID))
			}
		}
		if len(failedNodes) > 0 {
			ux.Logger.PrintToUser("Failed nodes: ")
			for node, err := range failedNodes {
				ux.Logger.PrintToUser(fmt.Sprintf("Failed to destroy node %s due to %s", node, err))
			}
			ux.Logger.PrintToUser("Destroy the above instance(s) on AWS console to prevent charges")
			return models.CloudConfig{}, fmt.Errorf("failed to destroy node(s) %s", failedNodes)
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

// checkRegions checks if the given regions are available in AWS.
// It returns list of invalid regions and error if any
func checkRegions(regions []string) ([]string, error) {
	const regionCheckerRegion = "us-east-1"
	invalidRegions := []string{}
	awsCloudRegionChecker, err := getAWSCloudCredentials(awsProfile, regionCheckerRegion)
	if err != nil {
		return invalidRegions, err
	}
	availableRegions, err := awsCloudRegionChecker.ListRegions()
	if err != nil {
		if isExpiredCredentialError(err) {
			printExpiredCredentialsOutput(awsProfile)
		}
		return invalidRegions, err
	}
	for _, region := range regions {
		if !slices.Contains(availableRegions, region) {
			invalidRegions = append(invalidRegions, region)
		}
	}
	return invalidRegions, nil
}
