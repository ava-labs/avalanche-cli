// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aws

import (
	"errors"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"sort"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var (
	ErrNoInstanceState = errors.New("unable to get instance state")
	ErrNoAddressFound  = errors.New("unable to get public IP address info on AWS")
)

// CheckKeyPairExists checks that key pair kpName exists in the AWS region and returns the key pair object
func CheckKeyPairExists(ec2Svc *ec2.EC2, kpName string) (bool, error) {
	keyPairInput := &ec2.DescribeKeyPairsInput{
		KeyNames: []*string{
			aws.String(kpName),
		},
	}

	_, err := ec2Svc.DescribeKeyPairs(keyPairInput)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidKeyPair.NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func GetUbuntuAMIID(ec2Svc *ec2.EC2) (string, error) {
	descriptionFilterValue := "Canonical, Ubuntu, 20.04 LTS, amd64*"
	imageInput := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{Name: aws.String("root-device-type"), Values: []*string{aws.String("ebs")}},
			{Name: aws.String("description"), Values: []*string{aws.String(descriptionFilterValue)}},
		},
		Owners: []*string{aws.String("self"), aws.String("amazon")},
	}
	images, err := ec2Svc.DescribeImages(imageInput)
	if err != nil {
		return "", err
	}
	if len(images.Images) == 0 {
		return "", fmt.Errorf("no amazon machine image found with the description %s", descriptionFilterValue)
	}
	// sort results by creation date
	sort.Slice(images.Images, func(i, j int) bool {
		return *images.Images[i].CreationDate > *images.Images[j].CreationDate
	})
	// get image with the latest creation date
	amiID := images.Images[0].ImageId
	return *amiID, nil
}

// CheckSecurityGroupExists checks that security group sgName exists in the AWS region and returns the security group object
func CheckSecurityGroupExists(ec2Svc *ec2.EC2, sgName string) (bool, *ec2.SecurityGroup, error) {
	sgInput := &ec2.DescribeSecurityGroupsInput{
		GroupNames: []*string{
			aws.String(sgName),
		},
	}

	sg, err := ec2Svc.DescribeSecurityGroups(sgInput)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidGroup.NotFound") {
			return false, &ec2.SecurityGroup{}, nil
		}
		return false, &ec2.SecurityGroup{}, err
	}
	return true, sg.SecurityGroups[0], nil
}

// CheckUserIPInSg checks that the user's current IP address is included in the whitelisted security group sg in AWS so that user can ssh into ec2 instance
func CheckUserIPInSg(sg *ec2.SecurityGroup, currentIP string, port int64) bool {
	for _, ipPermission := range sg.IpPermissions {
		for _, ip := range ipPermission.IpRanges {
			if strings.Contains(ip.String(), currentIP) {
				if *ipPermission.FromPort == port {
					return true
				}
			}
		}
	}
	return false
}

// GetInstancePublicIPs gets public IP(s) of EC2 instance(s) without elastic IP and returns a map
// with ec2 instance id as key and public ip as value
func GetInstancePublicIPs(ec2Svc *ec2.EC2, nodeIDs []string) (map[string]string, error) {
	nodeIDsInput := []*string{}
	for _, nodeID := range nodeIDs {
		nodeIDsInput = append(nodeIDsInput, aws.String(nodeID))
	}
	instanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: nodeIDsInput,
	}
	instanceResults, err := ec2Svc.DescribeInstances(instanceInput)
	if err != nil {
		return nil, err
	}
	reservations := instanceResults.Reservations
	if len(reservations) == 0 {
		return nil, ErrNoInstanceState
	}
	instanceIDToIP := make(map[string]string)
	for i := range reservations {
		instances := reservations[i].Instances
		if len(instances) == 0 {
			return nil, ErrNoInstanceState
		}
		instanceIDToIP[*instances[0].InstanceId] = *instances[0].PublicIpAddress
	}
	return instanceIDToIP, nil
}

// checkInstanceIsRunning checks that EC2 instance nodeID is running in AWS
func checkInstanceIsRunning(ec2Svc *ec2.EC2, nodeID string) (bool, error) {
	instanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(nodeID),
		},
	}
	nodeStatus, err := ec2Svc.DescribeInstances(instanceInput)
	if err != nil {
		return false, err
	}
	reservation := nodeStatus.Reservations
	if len(reservation) == 0 {
		return false, ErrNoInstanceState
	}
	instances := reservation[0].Instances
	if len(instances) == 0 {
		return false, ErrNoInstanceState
	}
	instanceStatus := instances[0].State.Name
	if *instanceStatus == constants.AWSCloudServerRunningState {
		return true, nil
	}
	return false, nil
}

func StopAWSNode(ec2Svc *ec2.EC2, nodeConfig models.NodeConfig, clusterName string) error {
	isRunning, err := checkInstanceIsRunning(ec2Svc, nodeConfig.NodeID)
	if err != nil {
		ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", nodeConfig.NodeID, err.Error()))
		return err
	}
	if !isRunning {
		noRunningNodeErr := fmt.Errorf("no running node with instance id %s is found in cluster %s", nodeConfig.NodeID, clusterName)
		return noRunningNodeErr
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Stopping node instance %s in cluster %s...", nodeConfig.NodeID, clusterName))
	return stopInstance(ec2Svc, nodeConfig.NodeID, nodeConfig.ElasticIP, true)
}

func stopInstance(ec2Svc *ec2.EC2, instanceID, publicIP string, releasePublicIP bool) error {
	input := &ec2.StopInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	}
	if _, err := ec2Svc.StopInstances(input); err != nil {
		return err
	}
	if releasePublicIP {
		describeAddressInput := &ec2.DescribeAddressesInput{
			Filters: []*ec2.Filter{
				{Name: aws.String("public-ip"), Values: []*string{aws.String(publicIP)}},
			},
		}
		addressOutput, err := ec2Svc.DescribeAddresses(describeAddressInput)
		if err != nil {
			return err
		}
		if len(addressOutput.Addresses) == 0 {
			return ErrNoAddressFound
		}
		releaseAddressInput := &ec2.ReleaseAddressInput{
			AllocationId: aws.String(*addressOutput.Addresses[0].AllocationId),
		}
		if _, err = ec2Svc.ReleaseAddress(releaseAddressInput); err != nil {
			return err
		}
	}
	return nil
}
