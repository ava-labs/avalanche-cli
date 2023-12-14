// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var (
	ErrNoInstanceState = errors.New("unable to get instance state")
	ErrNoAddressFound  = errors.New("unable to get public IP address info on AWS")
)

type AwsCloud struct {
	ec2Client *ec2.Client
	ctx       context.Context
}

// NewAwsCloud creates an AWS cloud
func NewAwsCloud(awsProfile, region string) (*AwsCloud, error) {
	ctx := context.Background()
	// Load session from shared config
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(awsProfile),
	)
	if err != nil {
		return nil, err
	}
	return &AwsCloud{
		ec2Client: ec2.NewFromConfig(cfg),
		ctx:       ctx,
	}, nil
}

// CreateSecurityGroup creates a security group
func (c *AwsCloud) CreateSecurityGroup(groupName, description string) (string, error) {
	createSGOutput, err := c.ec2Client.CreateSecurityGroup(c.ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(groupName),
		Description: aws.String(description),
	})
	if err != nil {
		return "", err
	}
	return *createSGOutput.GroupId, nil
}

// CheckSecurityGroupExists checks if the given security group exists
func (c *AwsCloud) CheckSecurityGroupExists(sgName string) (bool, types.SecurityGroup, error) {
	sgInput := &ec2.DescribeSecurityGroupsInput{
		GroupNames: []string{
			sgName,
		},
	}

	sg, err := c.ec2Client.DescribeSecurityGroups(c.ctx, sgInput)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidGroup.NotFound") {
			return false, types.SecurityGroup{}, nil
		}
		return false, types.SecurityGroup{}, err
	}
	return true, sg.SecurityGroups[0], nil
}

// AddSecurityGroupRule adds a rule to the given security group
func (c *AwsCloud) AddSecurityGroupRule(groupID, direction, protocol, ip string, port int32) error {
	if !strings.Contains(ip, "/") {
		ip = fmt.Sprintf("%s/32", ip) // add netmask /32 if missing
	}
	switch direction {
	case "ingress":
		if _, err := c.ec2Client.AuthorizeSecurityGroupIngress(c.ctx, &ec2.AuthorizeSecurityGroupIngressInput{
			GroupId: aws.String(groupID),
			IpPermissions: []types.IpPermission{
				{
					IpProtocol: aws.String(protocol),
					FromPort:   aws.Int32(port),
					ToPort:     aws.Int32(port),
					IpRanges: []types.IpRange{
						{
							CidrIp: aws.String(ip),
						},
					},
				},
			},
		}); err != nil {
			return err
		}
	case "egress":
		if _, err := c.ec2Client.AuthorizeSecurityGroupEgress(c.ctx, &ec2.AuthorizeSecurityGroupEgressInput{
			GroupId: aws.String(groupID),
			IpPermissions: []types.IpPermission{
				{
					IpProtocol: aws.String(protocol),
					FromPort:   aws.Int32(port),
					ToPort:     aws.Int32(port),
					IpRanges: []types.IpRange{
						{
							CidrIp: aws.String(ip),
						},
					},
				},
			},
		}); err != nil {
			return err
		}
	default:
		return errors.New("invalid direction")
	}
	return nil
}

// CreateEC2Instances creates EC2 instances
func (c *AwsCloud) CreateEC2Instances(count int, amiID, instanceType, keyName, securityGroupID string) ([]string, error) {
	runResult, err := c.ec2Client.RunInstances(c.ctx, &ec2.RunInstancesInput{
		ImageId:          aws.String(amiID),
		InstanceType:     types.InstanceType(instanceType),
		KeyName:          aws.String(keyName),
		SecurityGroupIds: []string{securityGroupID},
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(int32(count)),
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"), // ubuntu ami disk name
				Ebs: &types.EbsBlockDevice{
					VolumeSize: aws.Int32(1000),
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	switch len(runResult.Instances) {
	case 0:
		return nil, fmt.Errorf("no instances created")
	case count:
		instanceIDs := utils.Map(runResult.Instances, func(instance types.Instance) string {
			return *instance.InstanceId
		})
		return instanceIDs, nil
	default:
		return nil, fmt.Errorf("expected %d instances, got %d", count, len(runResult.Instances))
	}
}

// WaitForEC2Instances waits for the EC2 instances to be running
func (c *AwsCloud) WaitForEC2Instances(nodeIDs []string) error {
	instanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: nodeIDs,
	}
	// Custom waiter loop
	maxAttempts := 100
	delay := 1 * time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Describe instances to check their states
		result, err := c.ec2Client.DescribeInstances(c.ctx, instanceInput)
		if err != nil {
			time.Sleep(delay)
			continue
		}

		// Check if all instances are in the 'running' state
		allRunning := true
		for _, reservation := range result.Reservations {
			for _, instance := range reservation.Instances {
				if instance.State.Name != "running" {
					allRunning = false
					break
				}
			}
		}
		if allRunning {
			return nil
		}
		// If not all instances are running, wait and retry
		time.Sleep(delay)
	}
	return fmt.Errorf("timeout waiting for instances to be running")
}

// GetInstancePublicIPs returns a map from instance ID to public IP
func (c *AwsCloud) GetInstancePublicIPs(nodeIDs []string) (map[string]string, error) {
	instanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: nodeIDs,
	}
	instanceResults, err := c.ec2Client.DescribeInstances(c.ctx, instanceInput)
	if err != nil {
		return nil, err
	}
	reservations := instanceResults.Reservations
	if len(reservations) == 0 {
		return nil, ErrNoInstanceState
	}
	instanceIDToIP := make(map[string]string)
	for _, reservation := range instanceResults.Reservations {
		for _, instance := range reservation.Instances {
			instanceID := *instance.InstanceId
			publicIP := ""
			if instance.PublicIpAddress != nil {
				publicIP = *instance.PublicIpAddress
			}
			instanceIDToIP[instanceID] = publicIP
		}
	}
	return instanceIDToIP, nil
}

// checkInstanceIsRunning checks that EC2 instance nodeID is running in EC2
func (c *AwsCloud) checkInstanceIsRunning(nodeID string) (bool, error) {
	instanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{
			*aws.String(nodeID),
		},
	}
	nodeStatus, err := c.ec2Client.DescribeInstances(c.ctx, instanceInput)
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
	if instanceStatus == constants.AWSCloudServerRunningState {
		return true, nil
	}
	return false, nil
}

// StopAWSNode stops an EC2 instance with the given ID.
func (c *AwsCloud) StopAWSNode(nodeConfig models.NodeConfig, clusterName string) error {
	isRunning, err := c.checkInstanceIsRunning(nodeConfig.NodeID)
	if err != nil {
		ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", nodeConfig.NodeID, err.Error()))
		return err
	}
	if !isRunning {
		noRunningNodeErr := fmt.Errorf("no running node with instance id %s is found in cluster %s", nodeConfig.NodeID, clusterName)
		return noRunningNodeErr
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Stopping node instance %s in cluster %s...", nodeConfig.NodeID, clusterName))
	return c.StopInstance(nodeConfig.NodeID, nodeConfig.ElasticIP, true)
}

// StopInstance stops an EC2 instance with the given ID.
func (c *AwsCloud) StopInstance(instanceID, publicIP string, releasePublicIP bool) error {
	input := &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	}
	if _, err := c.ec2Client.StopInstances(c.ctx, input); err != nil {
		return err
	}
	if releasePublicIP {
		describeAddressInput := &ec2.DescribeAddressesInput{
			Filters: []types.Filter{
				{Name: aws.String("public-ip"), Values: []string{publicIP}},
			},
		}
		addressOutput, err := c.ec2Client.DescribeAddresses(c.ctx, describeAddressInput)
		if err != nil {
			return err
		}
		if len(addressOutput.Addresses) == 0 {
			return ErrNoAddressFound
		}
		releaseAddressInput := &ec2.ReleaseAddressInput{
			AllocationId: aws.String(*addressOutput.Addresses[0].AllocationId),
		}
		if _, err = c.ec2Client.ReleaseAddress(c.ctx, releaseAddressInput); err != nil {
			return err
		}
	}
	return nil
}

// CreateEIP creates an Elastic IP address.
func (c *AwsCloud) CreateEIP() (string, string, error) {
	if addr, err := c.ec2Client.AllocateAddress(c.ctx, &ec2.AllocateAddressInput{}); err != nil {
		if isEIPQuotaExceededError(err) {
			return "", "", fmt.Errorf("elastic IP quota exceeded: %w", err)
		}
		return "", "", err
	} else {
		return *addr.AllocationId, *addr.PublicIp, nil
	}
}

// AssociateEIP associates an Elastic IP address with an EC2 instance.
func (c *AwsCloud) AssociateEIP(instanceID, allocationID string) error {
	if _, err := c.ec2Client.AssociateAddress(c.ctx, &ec2.AssociateAddressInput{
		InstanceId:   aws.String(instanceID),
		AllocationId: aws.String(allocationID),
	}); err != nil {
		return err
	}
	return nil
}

// CreateAndDownloadKeyPair creates a new key pair and downloads the private key material to the specified file path.
func (c *AwsCloud) CreateAndDownloadKeyPair(keyName string, privateKeyFilePath string) error {
	createKeyPairOutput, err := c.ec2Client.CreateKeyPair(c.ctx, &ec2.CreateKeyPairInput{
		KeyName: aws.String(keyName),
	})
	if err != nil {
		return err
	}
	privateKeyMaterial := *createKeyPairOutput.KeyMaterial
	err = os.WriteFile(privateKeyFilePath, []byte(privateKeyMaterial), 0o600)
	if err != nil {
		return err
	}
	return nil
}

// SetupSecurityGroup sets up a security group for the AwsCloud instance.
func (c *AwsCloud) SetupSecurityGroup(ipAddress, securityGroupName string) (string, error) {
	sgID, err := c.CreateSecurityGroup(securityGroupName, "Allow SSH, AVAX HTTP outbound traffic")
	if err != nil {
		return "", err
	}
	if err := c.AddSecurityGroupRule(sgID, "ingress", "tcp", ipAddress, constants.SSHTCPPort); err != nil {
		return "", err
	}

	if err := c.AddSecurityGroupRule(sgID, "ingress", "tcp", ipAddress, constants.AvalanchegoAPIPort); err != nil {
		return "", err
	}
	if err := c.AddSecurityGroupRule(sgID, "ingress", "tcp", "0.0.0.0/0", constants.AvalanchegoP2PPort); err != nil {
		return "", err
	}
	return sgID, nil
}

// CheckUserIPInSg checks if the user IP is present in the SecurityGroup.
func CheckUserIPInSg(sg *types.SecurityGroup, currentIP string, port int32) bool {
	for _, ipPermission := range sg.IpPermissions {
		for _, ip := range ipPermission.IpRanges {
			if strings.Contains(*ip.CidrIp, currentIP) {
				if *ipPermission.FromPort == port {
					return true
				}
			}
		}
	}
	return false
}

// CheckKeyPairExists checks if the specified key pair exists in the AWS Cloud.
func (c *AwsCloud) CheckKeyPairExists(kpName string) (bool, error) {
	keyPairInput := &ec2.DescribeKeyPairsInput{
		KeyNames: []string{kpName},
	}
	_, err := c.ec2Client.DescribeKeyPairs(c.ctx, keyPairInput)
	if err != nil {
		if strings.Contains(err.Error(), "InvalidKeyPair.NotFound") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetUbuntuAMIID returns the ID of the latest Ubuntu Amazon Machine Image (AMI).
func (c *AwsCloud) GetUbuntuAMIID() (string, error) {
	descriptionFilterValue := "Canonical, Ubuntu, 20.04 LTS, amd64*"
	imageInput := &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{Name: aws.String("root-device-type"), Values: []string{"ebs"}},
			{Name: aws.String("description"), Values: []string{descriptionFilterValue}},
		},
		Owners: []string{"self", "amazon"},
	}
	images, err := c.ec2Client.DescribeImages(c.ctx, imageInput)
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

// isEIPQuotaExceededError checks if the error is related to exceeding the quota for Elastic IP addresses.
func isEIPQuotaExceededError(err error) bool {
	// You may need to adjust this function based on the actual error messages returned by AWS
	return err != nil && (utils.ContainsIgnoreCase(err.Error(), "limit exceeded") || utils.ContainsIgnoreCase(err.Error(), "elastic ip address limit exceeded"))
}
