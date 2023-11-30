// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package terraformaws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var (
	ErrNoInstanceState = errors.New("unable to get instance state")
	ErrNoAddressFound  = errors.New("unable to get public IP address info on AWS")
)

type awsCloud struct {
	ec2Client *ec2.Client
	ctx       context.Context
}

func NewAWSCloud(client *ec2.Client, ctx context.Context) (*awsCloud, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return &awsCloud{
		ec2Client: client,
		ctx:       ctx,
	}, nil
}

func (c *awsCloud) CreateSecurityGroup(groupName, description string) (string, error) {
	createSGOutput, err := c.ec2Client.CreateSecurityGroup(c.ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(groupName),
		Description: aws.String(description),
	})
	if err != nil {
		return "", err
	}
	return *createSGOutput.GroupId, nil
}

func (c *awsCloud) CheckSecurityGroupExists(sgName string) (bool, types.SecurityGroup, error) {
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

func (c *awsCloud) AddSecurityGroupRule(direction, groupID, protocol, ip string, port int32) error {
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

func (c *awsCloud) createEC2Instances(count int32, amiID, instanceType, keyName, securityGroupID string) ([]string, error) {
	runResult, err := c.ec2Client.RunInstances(c.ctx, &ec2.RunInstancesInput{
		ImageId:          aws.String(amiID),
		InstanceType:     types.InstanceType(instanceType),
		KeyName:          aws.String(keyName),
		SecurityGroupIds: []string{securityGroupID},
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(count),
	})
	if err != nil {
		return nil, err
	}
	switch len(runResult.Instances) {
	case 0:
		return nil, fmt.Errorf("no instances created")
	case int(count):
		instanceIDs := utils.Map(runResult.Instances, func(instance types.Instance) string {
			return *instance.InstanceId
		})
		return instanceIDs, nil
	default:
		return nil, fmt.Errorf("expected %d instances, got %d", count, len(runResult.Instances))
	}
}

func (c *awsCloud) GetInstancePublicIPs(nodeIDs []string) (map[string]string, error) {
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
	for i := range reservations {
		instances := reservations[i].Instances
		if len(instances) == 0 {
			return nil, ErrNoInstanceState
		}
		instanceIDToIP[*instances[0].InstanceId] = *instances[0].PublicIpAddress
	}
	return instanceIDToIP, nil
}

func (c *awsCloud) checkInstanceIsRunning(nodeID string) (bool, error) {
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

func (c *awsCloud) StopAWSNode(nodeConfig models.NodeConfig, clusterName string) error {
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
func (c *awsCloud) StopInstance(instanceID, publicIP string, releasePublicIP bool) error {
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

func (c *awsCloud) CreateEIP() (string, string, error) {
	if addr, err := c.ec2Client.AllocateAddress(c.ctx, &ec2.AllocateAddressInput{}); err != nil {
		return "", "", err
	} else {
		return *addr.AllocationId, *addr.PublicIp, nil
	}
}

func (c *awsCloud) AssociateEIP(instanceID, allocationID string) error {
	if _, err := c.ec2Client.AssociateAddress(c.ctx, &ec2.AssociateAddressInput{
		InstanceId:   aws.String(instanceID),
		AllocationId: aws.String(allocationID),
	}); err != nil {
		return err
	}
	return nil
}
func (c *awsCloud) CreateAndDownloadKeyPair(keyName string, privateKeyFilePath string) error {
	createKeyPairOutput, err := c.ec2Client.CreateKeyPair(c.ctx, &ec2.CreateKeyPairInput{
		KeyName: aws.String(keyName),
	})
	if err != nil {
		return err
	}
	privateKeyMaterial := *createKeyPairOutput.KeyMaterial
	err = os.WriteFile(privateKeyFilePath, []byte(privateKeyMaterial), 0600)
	if err != nil {
		return err
	}
	return nil
}

func (c *awsCloud) SetupSecurityGroup(ipAddress, securityGroupName string) error {
	inputIPAddress := ipAddress + "/32"
	sgID, err := c.CreateSecurityGroup(securityGroupName, "Allow SSH, AVAX HTTP outbound traffic")
	if err != nil {
		return err
	}
	if err := c.AddSecurityGroupRule("ingress", sgID, "tcp", inputIPAddress, constants.SSHTCPPort); err != nil {
		return err
	}

	if err := c.AddSecurityGroupRule("ingress", sgID, "tcp", inputIPAddress, constants.AvalanchegoAPIPort); err != nil {
		return err
	}
	if err := c.AddSecurityGroupRule("ingress", sgID, "tcp", "0.0.0.0/0", constants.AvalanchegoP2PPort); err != nil {
		return err
	}
	if err := c.AddSecurityGroupRule("egress", sgID, "-1", "0.0.0.0/0", constants.OutboundPort); err != nil {
		return err
	}
	return nil
}

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

func (c *awsCloud) CheckKeyPairExists(kpName string) (bool, error) {
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

func (c *awsCloud) GetUbuntuAMIID() (string, error) {
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
