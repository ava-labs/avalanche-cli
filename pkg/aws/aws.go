// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aws

import (
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func CheckKeyPairExists(ec2Svc *ec2.EC2, kpName string) (bool, error) {
	keyPairInput := &ec2.DescribeKeyPairsInput{
		KeyNames: []*string{
			aws.String(kpName),
		},
	}

	// Call to get detailed information on each instance
	_, err := ec2Svc.DescribeKeyPairs(keyPairInput)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidKeyPair.NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

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

func CheckCurrentIPInSg(sg *ec2.SecurityGroup, currentIP string) (bool, bool) {
	var ipInTCP bool
	var ipInHTTP bool
	for _, ip := range sg.IpPermissions {
		if *ip.FromPort == constants.TCPPort || *ip.FromPort == constants.HTTPPort {
			for _, cidrIP := range ip.IpRanges {
				if strings.Contains(cidrIP.String(), currentIP) {
					if *ip.FromPort == constants.TCPPort {
						ipInTCP = true
					} else if *ip.FromPort == constants.HTTPPort {
						ipInHTTP = true
					}
					break
				}
			}
		}
	}
	return ipInTCP, ipInHTTP
}
