// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"context"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/ids"
	"os"
	"os/user"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spf13/cobra"
)

func newJoinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [subnetName]",
		Short: "Create a new subnet configuration",
		Long: `The subnet create command builds a new genesis file to configure your Subnet.
By default, the command runs an interactive wizard. It walks you through
all the steps you need to create your first Subnet.

The tool supports deploying Subnet-EVM, and custom VMs. You
can create a custom, user-generated genesis with a custom VM by providing
the path to your genesis and VM binaries with the --genesis and --vm flags.

By default, running the command with a subnetName that already exists
causes the command to fail. If you’d like to overwrite an existing
configuration, pass the -f flag.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         joinSubnet,
	}

	return cmd
}

func CheckNodeIsBootstrapped(subnetID ids.ID) error {
	api := constants.FujiAPIEndpoint
	infoClient := info.NewClient(api)
	ctx, cancel := context.WithTimeout(context.Background(), constants.RequestTimeout)
	defer cancel()
	_, err := infoClient.IsBootstrapped(subnetID)
	return err
}

func joinSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]

	err := removeExistingTerraformFiles()
	if err != nil {
		return err
	}
	usr, _ := user.Current()
	keyPairName := usr.Username + "-avalanche-cli"
	certName := keyPairName + "-kp.pem"
	securityGroupName := keyPairName + "-sg"
	region := "us-east-2"
	ami := "ami-0430580de6244e02e"

	hclFile := hclwrite.NewEmptyFile()
	tfFile, err := os.Create("node_config.tf")
	if err != nil {
		return err
	}
	rootBody := hclFile.Body()
	creds := credentials.NewSharedCredentials("", "default")
	credValue, err := creds.Get()
	if err != nil {
		printNoCredentialsOutput()
		return err
	}
	err = setCloudCredentials(rootBody, credValue.AccessKeyID, credValue.SecretAccessKey, region)
	if err != nil {
		return err
	}
	// Load session from shared config
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	})
	if err != nil {
		return err
	}

	// Create new EC2 client
	ec2Svc := ec2.New(sess)
	var useExistingKeyPair bool
	keyPairExists, err := checkKeyPairExists(ec2Svc, keyPairName)
	if err != nil {
		return err
	}
	if !keyPairExists {
		setKeyPair(rootBody, keyPairName, certName)
	} else {
		certFilePath, err := getCertFilePath(certName)
		if err != nil {
			return err
		}
		if checkCertInSSHDir(certFilePath) {
			useExistingKeyPair = true
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Default Key Pair named %s already exists", keyPairName))
			keyPairName, err = getNewKeyPairName(ec2Svc)
			if err != nil {
				return err
			}
			certName = keyPairName + "-kp.pem"
			setKeyPair(rootBody, keyPairName, certName)
		}
	}
	securityGroupExists, sg, err := checkSecurityGroupExists(ec2Svc, securityGroupName)
	if err != nil {
		return err
	}
	userIPAddress, err := getIPAddress()
	if err != nil {
		return err
	}
	if !securityGroupExists {
		setSecurityGroup(rootBody, userIPAddress, securityGroupName)
	} else {
		ipInTCP, ipInHTTP := checkCurrentIPInSg(sg, userIPAddress)
		setSecurityGroupRule(rootBody, userIPAddress, *sg.GroupId, ipInTCP, ipInHTTP)
	}
	setElasticIP(rootBody)
	setUpInstance(rootBody, securityGroupName, useExistingKeyPair, keyPairName, ami)
	setOutput(rootBody)
	_, err = tfFile.Write(hclFile.Bytes())
	if err != nil {
		return err
	}
	instanceID, elasticIP, err := runTerraform()
	if err != nil {
		return err
	}
	certFilePath, err := getCertFilePath(certName)
	if err != nil {
		return err
	}

	if !useExistingKeyPair {
		err = handleCerts(certName)
		if err != nil {
			return err
		}
	}

	inventoryPath := "inventories/" + clusterName
	if err := createAnsibleHostInventory(inventoryPath, elasticIP, certFilePath); err != nil {
		return err
	}
	time.Sleep(5 * time.Second)

	if err := runAnsiblePlaybook(inventoryPath); err != nil {
		return err
	}
	err = createNodeConfig(instanceID, region, ami, keyPairName, certFilePath, securityGroupName, elasticIP, clusterName)
	if err != nil {
		return err
	}
	PrintResults(instanceID, elasticIP, certFilePath, region)
	return nil
}
