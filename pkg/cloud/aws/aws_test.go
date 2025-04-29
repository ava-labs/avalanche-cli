// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TestCheckIPInSg tests the CheckIPInSg function
func TestCheckIPInSg(t *testing.T) {
	port80 := int32(80)
	port22 := int32(22)
	port443 := int32(443)
	sg := &types.SecurityGroup{
		IpPermissions: []types.IpPermission{
			{
				FromPort: &port80,
				IpRanges: []types.IpRange{
					{CidrIp: aws.String("192.168.1.0/24")},
					{CidrIp: aws.String("10.0.0.0/16")},
					{CidrIp: aws.String("1.1.1.1/32")},
				},
			},
			{
				FromPort: &port443,
				IpRanges: []types.IpRange{
					{CidrIp: aws.String("172.16.0.0/16")},
				},
			},
			{
				FromPort: &port22,
				IpRanges: []types.IpRange{
					{CidrIp: aws.String("0.0.0.0/0")},
				},
			},
		},
	}

	// ip is present in SG
	present := CheckIPInSg(sg, "192.168.1.5", 80)
	if !present {
		t.Errorf("Expected IP to be present in SecurityGroup")
	}

	// ip is not present in SG
	notPresent := CheckIPInSg(sg, "192.168.2.5", 80)
	if notPresent {
		t.Errorf("Expected IP not to be present in SecurityGroup")
	}

	// invalid IP
	invalidIP := CheckIPInSg(sg, "invalid_ip", 80)
	if invalidIP {
		t.Errorf("Expected invalid IP not to be present in SecurityGroup")
	}

	// ip present in SG but with wrong port
	wrongPort := CheckIPInSg(sg, "10.0.1.5", 443)
	if wrongPort {
		t.Errorf("Expected IP to be present in SecurityGroup but with wrong port")
	}

	// ip is not present in any CIDR range in SG
	outsideRange := CheckIPInSg(sg, "172.17.0.5", 443)
	if outsideRange {
		t.Errorf("Expected IP not to be present in any CIDR range in SecurityGroup")
	}

	// current IP and security group both have 0.0.0.0/0 but with port 22
	bothAnyDifferentPort := CheckIPInSg(sg, "0.0.0.0/0", 22)
	if !bothAnyDifferentPort {
		t.Errorf("Expected both 0.0.0.0/0 IP addresses to match with port 22")
	}
	bothAnyDifferentPortNoMask := CheckIPInSg(sg, "0.0.0.0", 22)
	if !bothAnyDifferentPortNoMask {
		t.Errorf("Expected both 0.0.0.0/0 IP addresses to match with port 22")
	}
	fullButWrongPort := CheckIPInSg(sg, "0.0.0.0/0", 23)
	if fullButWrongPort {
		t.Errorf("Expected both 0.0.0.0/0 23 IP addresses to match 0.0.0.0/0 with port 22")
	}
	fullButWrongPortNoMask := CheckIPInSg(sg, "0.0.0.0", 23)
	if fullButWrongPortNoMask {
		t.Errorf("Expected both 0.0.0.0 23 IP addresses to match 0.0.0.0/0 with port 22")
	}
	someIPAndFullAccess := CheckIPInSg(sg, "1.1.1.1", 22)
	if !someIPAndFullAccess {
		t.Errorf("Expected both 0.0.0.0 23 IP addresses to match 0.0.0.0/0 with port 22")
	}

	// current IP and security group both have 1.1.1.1/32
	bothSpecific := CheckIPInSg(sg, "1.1.1.1/32", 80)
	if !bothSpecific {
		t.Errorf("Expected both 1.1.1.1/32 IP addresses to match")
	}
}
