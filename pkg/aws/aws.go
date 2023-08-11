// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
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
func GetAMIID(ec2Svc *ec2.EC2) (string, error) {
	descriptionFilterValue := "Canonical, Ubuntu, 20.04 LTS, amd64 focal image build on 2023-05-17"
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
