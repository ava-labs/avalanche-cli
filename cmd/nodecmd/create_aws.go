// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/hashicorp/hcl/v2/hclwrite"

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
	ux.Logger.PrintToUser("What do you want to name your key pair?")
	for {
		newKeyPairName, err := app.Prompt.CaptureString("Key Pair Name")
		if err != nil {
			return "", err
		}
		keyPairExists, err := awsAPI.CheckKeyPairExists(ec2Svc, newKeyPairName)
		if err != nil {
			return "", err
		}
		if !keyPairExists {
			return newKeyPairName, nil
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Key Pair named %s already exists", newKeyPairName))
	}
}

// getAWSCloudCredentials gets AWS account credentials defined in .aws dir in user home dir
func getAWSCloudCredentials(region, awsCommand string) (*session.Session, error) {
	if awsCommand == constants.StopAWSNode {
		if err := requestStopAWSNodeAuth(); err != nil {
			return &session.Session{}, err
		}
	} else if awsCommand == constants.CreateAWSNode {
		if err := requestAWSAccountAuth(); err != nil {
			return &session.Session{}, err
		}
	}
	creds := credentials.NewSharedCredentials("", constants.AWSDefaultCredential)
	if _, err := creds.Get(); err != nil {
		printNoCredentialsOutput()
		return &session.Session{}, err
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

func getAWSCloudConfig() (*ec2.EC2, string, string, error) {
	usEast1 := "us-east-1"
	usEast2 := "us-east-2"
	usWest1 := "us-west-1"
	usWest2 := "us-west-2"
	customRegion := "Choose custom region (list of regions available at https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html)"
	region, err := app.Prompt.CaptureList(
		"Which AWS region do you want to set up your node in?",
		[]string{usEast1, usEast2, usWest1, usWest2, customRegion},
	)
	if err != nil {
		return nil, "", "", err
	}
	if region == customRegion {
		region, err = app.Prompt.CaptureString("Which AWS region do you want to set up your node in?")
		if err != nil {
			return nil, "", "", err
		}
	}
	sess, err := getAWSCloudCredentials(region, constants.CreateAWSNode)
	if err != nil {
		return nil, "", "", err
	}
	ec2Svc := ec2.New(sess)
	ami, err := awsAPI.GetUbuntuAMIID(ec2Svc)
	if err != nil {
		return nil, "", "", err
	}
	return ec2Svc, region, ami, nil
}

// createEC2Instances creates terraform .tf file and runs terraform exec function to create ec2 instances
func createEC2Instances(rootBody *hclwrite.Body,
	ec2Svc *ec2.EC2,
	hclFile *hclwrite.File,
	region,
	ami,
	certName,
	keyPairName,
	securityGroupName string,
) ([]string, []string, string, string, error) {
	if err := terraformaws.SetCloudCredentials(rootBody, region); err != nil {
		return nil, nil, "", "", err
	}
	numNodes, err := app.Prompt.CaptureUint32("How many nodes do you want to set up on AWS?")
	if err != nil {
		return nil, nil, "", "", err
	}
	ux.Logger.PrintToUser("Creating new EC2 instance(s) on AWS...")
	var useExistingKeyPair bool
	keyPairExists, err := awsAPI.CheckKeyPairExists(ec2Svc, keyPairName)
	if err != nil {
		return nil, nil, "", "", err
	}
	certInSSHDir, err := app.CheckCertInSSHDir(certName)
	if err != nil {
		return nil, nil, "", "", err
	}
	if !keyPairExists {
		if !certInSSHDir {
			ux.Logger.PrintToUser(fmt.Sprintf("Creating new key pair %s in AWS", keyPairName))
			terraformaws.SetKeyPair(rootBody, keyPairName, certName)
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists on your .ssh directory but not on AWS", keyPairName))
			ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in AWS", keyPairName))
			certName, keyPairName, err = promptKeyPairName(ec2Svc)
			if err != nil {
				return nil, nil, "", "", err
			}
			terraformaws.SetKeyPair(rootBody, keyPairName, certName)
		}
	} else {
		if certInSSHDir {
			ux.Logger.PrintToUser(fmt.Sprintf("Using existing key pair %s in AWS", keyPairName))
			useExistingKeyPair = true
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists in AWS", keyPairName))
			ux.Logger.PrintToUser(fmt.Sprintf("We need to create a new Key Pair in AWS as we can't find Key Pair named %s in your .ssh directory", keyPairName))
			certName, keyPairName, err = promptKeyPairName(ec2Svc)
			if err != nil {
				return nil, nil, "", "", err
			}
			terraformaws.SetKeyPair(rootBody, keyPairName, certName)
		}
	}
	securityGroupExists, sg, err := awsAPI.CheckSecurityGroupExists(ec2Svc, securityGroupName)
	if err != nil {
		return nil, nil, "", "", err
	}
	userIPAddress, err := getIPAddress()
	if err != nil {
		return nil, nil, "", "", err
	}
	if !securityGroupExists {
		ux.Logger.PrintToUser(fmt.Sprintf("Creating new security group %s in AWS", securityGroupName))
		terraformaws.SetSecurityGroup(rootBody, userIPAddress, securityGroupName)
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("Using existing security group %s in AWS", securityGroupName))
		ipInTCP := awsAPI.CheckUserIPInSg(sg, userIPAddress, constants.SSHTCPPort)
		ipInHTTP := awsAPI.CheckUserIPInSg(sg, userIPAddress, constants.AvalanchegoAPIPort)
		terraformaws.SetSecurityGroupRule(rootBody, userIPAddress, *sg.GroupId, ipInTCP, ipInHTTP)
	}
	if useStaticIP {
		terraformaws.SetElasticIPs(rootBody, numNodes)
	}
	terraformaws.SetupInstances(rootBody, securityGroupName, useExistingKeyPair, keyPairName, ami, numNodes)
	terraformaws.SetOutput(rootBody, useStaticIP)
	err = app.CreateTerraformDir()
	if err != nil {
		return nil, nil, "", "", err
	}
	err = terraform.SaveConf(app.GetTerraformDir(), hclFile)
	if err != nil {
		return nil, nil, "", "", err
	}
	instanceIDs, elasticIPs, err := terraformaws.RunTerraform(app.GetTerraformDir(), useStaticIP)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("%s, %s", constants.ErrCreatingAWSNode, err)
	}
	ux.Logger.PrintToUser("New EC2 instance(s) successfully created in AWS!")
	if !useExistingKeyPair {
		// takes the cert file downloaded from AWS through terraform and moves it to .ssh directory
		err = addCertToSSH(certName)
		if err != nil {
			return nil, nil, "", "", err
		}
	}
	sshCertPath, err := app.GetSSHCertFilePath(certName)
	if err != nil {
		return nil, nil, "", "", err
	}
	return instanceIDs, elasticIPs, sshCertPath, keyPairName, nil
}

func createAWSInstance(ec2Svc *ec2.EC2, region, ami string, usr *user.User) (CloudConfig, error) {
	prefix := usr.Username + "-" + region + constants.AvalancheCLISuffix
	certName := prefix + "-" + region + constants.CertSuffix
	securityGroupName := prefix + "-" + region + constants.AWSSecurityGroupSuffix
	hclFile, rootBody, err := terraform.InitConf()
	if err != nil {
		return CloudConfig{}, nil
	}

	// Create new EC2 instances
	instanceIDs, elasticIPs, certFilePath, keyPairName, err := createEC2Instances(rootBody, ec2Svc, hclFile, region, ami, certName, prefix, securityGroupName)
	if err != nil {
		if err.Error() == constants.EIPLimitErr {
			ux.Logger.PrintToUser("Failed to create AWS cloud server, please try creating again in a different region")
		} else {
			ux.Logger.PrintToUser("Failed to create AWS cloud server")
		}
		if strings.Contains(err.Error(), constants.ErrCreatingAWSNode) {
			// we stop created instances so that user doesn't pay for unused EC2 instances
			instanceIDs, instanceIDErr := terraformaws.GetInstanceIDs(app.GetTerraformDir())
			if instanceIDErr != nil {
				return CloudConfig{}, instanceIDErr
			}
			failedNodes := []string{}
			nodeErrors := []error{}
			for _, instanceID := range instanceIDs {
				ux.Logger.PrintToUser(fmt.Sprintf("Stopping AWS cloud server %s...", instanceID))
				if stopErr := awsAPI.StopInstance(ec2Svc, instanceID, "", false); stopErr != nil {
					failedNodes = append(failedNodes, instanceID)
					nodeErrors = append(nodeErrors, stopErr)
				}
				ux.Logger.PrintToUser(fmt.Sprintf("AWS cloud server instance %s stopped", instanceID))
			}
			if len(failedNodes) > 0 {
				ux.Logger.PrintToUser("Failed nodes: ")
				for i, node := range failedNodes {
					ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, nodeErrors[i]))
				}
				ux.Logger.PrintToUser("Stop the above instance(s) on AWS console to prevent charges")
				return CloudConfig{}, fmt.Errorf("failed to stop node(s) %s", failedNodes)
			}
		}
		return CloudConfig{}, err
	}
	awsCloudConfig := CloudConfig{
		instanceIDs,
		elasticIPs,
		region,
		keyPairName,
		securityGroupName,
		certFilePath,
		ami,
	}
	return awsCloudConfig, nil
}

func requestAWSAccountAuth() error {
	ux.Logger.PrintToUser("Do you authorize Avalanche-CLI to access your AWS account to set-up your Avalanche Validator node?")
	ux.Logger.PrintToUser("Please note that you will be charged for AWS usage.")
	ux.Logger.PrintToUser("By clicking yes, you are authorizing Avalanche-CLI to:")
	ux.Logger.PrintToUser("- Set up EC2 instance(s) and other components (such as security groups, key pairs and elastic IPs)")
	ux.Logger.PrintToUser("- Set up the EC2 instance(s) to validate the Avalanche Primary Network")
	ux.Logger.PrintToUser("- Set up the EC2 instance(s) to validate Subnets")
	yes, err := app.Prompt.CaptureYesNo("I authorize Avalanche-CLI to access my AWS account")
	if err != nil {
		return err
	}
	if !yes {
		return errors.New("user did not give authorization to Avalanche-CLI to access AWS account")
	}
	return nil
}

func requestStopAWSNodeAuth() error {
	ux.Logger.PrintToUser("Do you authorize Avalanche-CLI to access your AWS account to stop your Avalanche Validator node?")
	ux.Logger.PrintToUser("By clicking yes, you are authorizing Avalanche-CLI to:")
	ux.Logger.PrintToUser("- Stop EC2 instance(s) and other components (such as elastic IPs)")
	yes, err := app.Prompt.CaptureYesNo("I authorize Avalanche-CLI to access my AWS account")
	if err != nil {
		return err
	}
	if !yes {
		return errors.New("user did not give authorization to Avalanche-CLI to access AWS account")
	}
	return nil
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
