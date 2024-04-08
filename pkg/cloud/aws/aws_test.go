package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TestCheckIPInSg tests the CheckIPInSg function
func TestCheckIPInSg(t *testing.T) {
	port80 := int32(80)
	port443 := int32(443)
	sg := &types.SecurityGroup{
		IpPermissions: []types.IpPermission{
			{
				FromPort: &port80,
				IpRanges: []types.IpRange{
					{CidrIp: aws.String("192.168.1.0/24")},
					{CidrIp: aws.String("10.0.0.0/16")},
				},
			},
			{
				FromPort: &port443,
				IpRanges: []types.IpRange{
					{CidrIp: aws.String("172.16.0.0/16")},
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
}
