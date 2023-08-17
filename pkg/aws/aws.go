// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aws

import (
	"errors"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var ErrNoInstanceState = errors.New("unable to get instance state")
var ErrNoAddressFound = errors.New("unable to get public IP address info on AWS")

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

// CheckInstanceIsRunning checks that EC2 instance nodeID is running in AWS
func CheckInstanceIsRunning(ec2Svc *ec2.EC2, nodeID string) (bool, error) {
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

func StopInstance(ec2Svc *ec2.EC2, instanceID, publicIP string) error {
	input := &ec2.StopInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	}
	if _, err := ec2Svc.StopInstances(input); err != nil {
		return err
	}
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
	return nil
}
