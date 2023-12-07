// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"golang.org/x/exp/maps"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	"github.com/ava-labs/avalanche-cli/pkg/terraform"
	terraformaws "github.com/ava-labs/avalanche-cli/pkg/terraform/aws"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func getNewKeyPairName(ec2Svc *ec2.EC2) (string, error) {
	newKeyPairName := cmdLineAlternativeKeyPairName
	for {
		if newKeyPairName != "" {
			keyPairExists, err := awsAPI.CheckKeyPairExists(ec2Svc, newKeyPairName)
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
func getAWSCloudCredentials(awsProfile, region string) (*session.Session, error) {
	if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.AWSCloudService) != nil) {
		return nil, fmt.Errorf("cloud access is required")
	}
	// use env variables first and fallback to shared config
	creds := credentials.NewEnvCredentials()
	if _, err := creds.Get(); err != nil {
		creds = credentials.NewSharedCredentials("", awsProfile)
		if _, err := creds.Get(); err != nil {
			printNoCredentialsOutput(awsProfile)
			return &session.Session{}, err
		}
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

func getAWSCloudConfig(awsProfile string) ([]string, []int, map[string]*ec2.EC2, map[string]string, error) {
	finalRegions := map[string]int{}
	switch {
	case len(numNodes) != len(utils.Unique(cmdLineRegion)):
		return nil, nil, nil, nil, fmt.Errorf("number of nodes and regions should be the same")
	case len(cmdLineRegion) == 0 && len(numNodes) == 0:
		var err error
		finalRegions, err = getRegionsNodeNum(constants.AWSCloudService)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	default:
		for i, region := range cmdLineRegion {
			finalRegions[region] = numNodes[i]
		}
	}
	ec2SvcMap := map[string]*ec2.EC2{}
	amiMap := map[string]string{}
	for region := range finalRegions {
		sess, err := getAWSCloudCredentials(awsProfile, region)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		ec2SvcMap[region] = ec2.New(sess)
		amiMap[region], err = awsAPI.GetUbuntuAMIID(ec2SvcMap[region])
		if err != nil {
			if strings.Contains(err.Error(), "RequestExpired: Request has expired") {
				printExpiredCredentialsOutput(awsProfile)
			}
			return nil, nil, nil, nil, err
		}
	}
	return maps.Keys(finalRegions), maps.Values(finalRegions), ec2SvcMap, amiMap, nil
}

// createEC2Instances creates terraform .tf file and runs terraform exec function to create ec2 instances
func createEC2Instances(rootBody *hclwrite.Body,
	ec2Svc map[string]*ec2.EC2,
	hclFile *hclwrite.File,
	numNodes []int,
	awsProfile string,
	regions []string,
	ami map[string]string,
	regionConf map[string]models.RegionConfig,
) (map[string][]string, map[string][]string, map[string]string, map[string]string, error) {
	if err := terraformaws.SetCloudCredentials(rootBody, awsProfile, regions); err != nil {
		return nil, nil, nil, nil, err
	}

	for i, region := range regions {
		if entry, ok := regionConf[region]; ok {
			entry.NumNodes = numNodes[i]
			regionConf[region] = entry
		}
	}

	ux.Logger.PrintToUser("Creating new EC2 instance(s) on AWS...")
	userIPAddress, err := getIPAddress()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	useExistingKeyPair := map[string]bool{}
	keyPairName := map[string]string{}
	for _, region := range regions {
		keyPairExists, err := awsAPI.CheckKeyPairExists(ec2Svc[region], regionConf[region].Prefix)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		certInSSHDir, err := app.CheckCertInSSHDir(regionConf[region].CertName)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		certName := regionConf[region].CertName
		keyPairName[region] = regionConf[region].Prefix
		securityGroupName := regionConf[region].SecurityGroupName
		if !keyPairExists {
			if !certInSSHDir {
				ux.Logger.PrintToUser(fmt.Sprintf("Creating new key pair %s in AWS[%s]", keyPairName, region))
				terraformaws.SetKeyPair(rootBody, region, regionConf[region].Prefix, certName)
			} else {
				ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists on your .ssh directory but not on AWS", regionConf[region].Prefix))
				ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in AWS[%s]", regionConf[region].Prefix, region))
				certName, keyPairName[region], err = promptKeyPairName(ec2Svc[region])
				if err != nil {
					return nil, nil, nil, nil, err
				}
				terraformaws.SetKeyPair(rootBody, region, keyPairName[region], certName)
			}
		} else {
			if certInSSHDir {
				ux.Logger.PrintToUser(fmt.Sprintf("Using existing key pair %s in AWS[%s]", keyPairName, region))
				useExistingKeyPair[region] = true
			} else {
				ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists in AWS[%s]", keyPairName, region))
				ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in your .ssh directory", keyPairName))
				certName, keyPairName[region], err = promptKeyPairName(ec2Svc[region])
				if err != nil {
					return nil, nil, nil, nil, err
				}
				terraformaws.SetKeyPair(rootBody, region, keyPairName[region], certName)
			}
		}
		securityGroupExists, sg, err := awsAPI.CheckSecurityGroupExists(ec2Svc[region], regionConf[region].SecurityGroupName)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if !securityGroupExists {
			ux.Logger.PrintToUser(fmt.Sprintf("Creating new security group %s in AWS[%s]", securityGroupName, region))
			terraformaws.SetSecurityGroup(rootBody, region, userIPAddress, securityGroupName)
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Using existing security group %s in AWS[%s]", securityGroupName, region))
			ipInTCP := awsAPI.CheckUserIPInSg(sg, userIPAddress, constants.SSHTCPPort)
			ipInHTTP := awsAPI.CheckUserIPInSg(sg, userIPAddress, constants.AvalanchegoAPIPort)
			terraformaws.SetSecurityGroupRule(rootBody, region, userIPAddress, *sg.GroupId, ipInTCP, ipInHTTP)
		}
		if useStaticIP {
			terraformaws.SetElasticIPs(rootBody, region, regionConf[region].NumNodes)
		}
		terraformaws.SetupInstances(rootBody, region, securityGroupName, useExistingKeyPair[region], keyPairName[region], ami[region], regionConf[region].NumNodes, regionConf[region].InstanceType)
	}
	terraformaws.SetOutput(rootBody, regions, useStaticIP)

	err = app.CreateTerraformDir()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	err = terraform.SaveConf(app.GetTerraformDir(), hclFile)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	instanceIDs, elasticIPs, err := terraformaws.RunTerraform(app.GetTerraformDir(), regions, useStaticIP)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%s, %w", constants.ErrCreatingAWSNode, err)
	}
	ux.Logger.PrintToUser("New EC2 instance(s) successfully created in AWS!")
	sshCertPath := map[string]string{}
	for _, region := range regions {
		if !useExistingKeyPair[region] {
			// takes the cert file downloaded from AWS through terraform and moves it to .ssh directory
			err = addCertToSSH(regionConf[region].CertName)
			if err != nil {
				return nil, nil, nil, nil, err
			}
		}
		sshCertPath[region], err = app.GetSSHCertFilePath(regionConf[region].CertName)
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}
	return instanceIDs, elasticIPs, sshCertPath, keyPairName, nil
}

func createAWSInstances(
	ec2Svc map[string]*ec2.EC2,
	nodeType string, numNodes []int,
	awsProfile string,
	regions []string,
	ami map[string]string,
	usr *user.User) (
	models.CloudConfig, error,
) {
	regionConf := map[string]models.RegionConfig{}

	for _, region := range regions {
		prefix := usr.Username + "-" + region + constants.AvalancheCLISuffix
		regionConf[region] = models.RegionConfig{
			Prefix:            prefix,
			CertName:          prefix + "-" + region + constants.CertSuffix,
			SecurityGroupName: prefix + "-" + region + constants.AWSSecurityGroupSuffix,
			InstanceType:      nodeType,
		}
	}

	hclFile, rootBody, err := terraform.InitConf()
	if err != nil {
		return models.CloudConfig{}, nil
	}

	// Create new EC2 instances
	instanceIDs, elasticIPs, certFilePath, keyPairName, err := createEC2Instances(rootBody, ec2Svc, hclFile, numNodes, awsProfile, regions, ami, regionConf)
	if err != nil {
		if strings.Contains(err.Error(), terraformaws.TerraformInitErrorStr) {
			return models.CloudConfig{}, err
		}
		if err.Error() == constants.EIPLimitErr {
			ux.Logger.PrintToUser("Failed to create AWS cloud server(s), please try creating again in a different region")
		} else {
			ux.Logger.PrintToUser("Failed to create AWS cloud server(s)")
		}
		if strings.Contains(err.Error(), constants.ErrCreatingAWSNode) {
			// we stop created instances so that user doesn't pay for unused EC2 instances
			ux.Logger.PrintToUser("Stopping all created AWS instances due to error to prevent charge for unused AWS instances...")
			instanceIDs, instanceIDErr := terraformaws.GetInstanceIDs(app.GetTerraformDir(), regions)
			if instanceIDErr != nil {
				return models.CloudConfig{}, instanceIDErr
			}
			failedNodes := map[string]error{}
			for region, regionInstanceID := range instanceIDs {
				for _, instanceID := range regionInstanceID {
					ux.Logger.PrintToUser(fmt.Sprintf("Stopping AWS cloud server %s...", instanceID))
					if stopErr := awsAPI.StopInstance(ec2Svc[region], instanceID, "", false); stopErr != nil {
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
		}
		return nil, err
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
