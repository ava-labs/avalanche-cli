// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aws

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
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
	ErrNoInstanceState         = errors.New("unable to get instance state")
	ErrNoAddressFound          = errors.New("unable to get public IP address info on AWS")
	ErrNodeNotFoundToBeRunning = errors.New("node not found to be running")
)

type AwsCloud struct {
	ec2Client *ec2.Client
	ctx       context.Context
}

// NewAwsCloud creates an AWS cloud
func NewAwsCloud(awsProfile, region string) (*AwsCloud, error) {
	var (
		cfg aws.Config
		err error
	)
	ctx := context.Background()
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		// Load session from env variables
		cfg, err = config.LoadDefaultConfig(
			ctx,
			config.WithRegion(region),
		)
	} else {
		// Load session from profile in config file
		cfg, err = config.LoadDefaultConfig(
			ctx,
			config.WithRegion(region),
			config.WithSharedConfigProfile(awsProfile),
		)
	}
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

// DeleteSecurityGroupRule removes a rule from the given security group
func (c *AwsCloud) DeleteSecurityGroupRule(groupID, direction, protocol, ip string, port int32) error {
	if !strings.Contains(ip, "/") {
		ip = fmt.Sprintf("%s/32", ip) // add netmask /32 if missing
	}
	switch direction {
	case "ingress":
		if _, err := c.ec2Client.RevokeSecurityGroupIngress(c.ctx, &ec2.RevokeSecurityGroupIngressInput{
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
		if _, err := c.ec2Client.RevokeSecurityGroupEgress(c.ctx, &ec2.RevokeSecurityGroupEgressInput{
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
func (c *AwsCloud) CreateEC2Instances(prefix string, count int, amiID, instanceType, keyName, securityGroupID string, forMonitoring bool, iops, throughput int, volumeType types.VolumeType, volumeSize int) ([]string, error) {
	var diskVolumeSize int32
	if forMonitoring {
		diskVolumeSize = constants.MonitoringCloudServerStorageSize
	} else {
		diskVolumeSize = int32(volumeSize)
	}
	ebsValue := &types.EbsBlockDevice{
		VolumeSize:          aws.Int32(diskVolumeSize),
		VolumeType:          volumeType,
		DeleteOnTermination: aws.Bool(true),
	}
	if volumeType == types.VolumeTypeGp3 {
		ebsValue.Throughput = aws.Int32(int32(throughput))
		ebsValue.Iops = aws.Int32(int32(iops))
	} else if volumeType == types.VolumeTypeIo2 || volumeType == types.VolumeTypeIo1 {
		ebsValue.Iops = aws.Int32(int32(iops))
	}

	runResult, err := c.ec2Client.RunInstances(c.ctx, &ec2.RunInstancesInput{
		ImageId:          aws.String(amiID),
		InstanceType:     types.InstanceType(instanceType),
		KeyName:          aws.String(keyName),
		SecurityGroupIds: []string{securityGroupID},
		MinCount:         aws.Int32(int32(count)),
		MaxCount:         aws.Int32(int32(count)),
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"), // ubuntu ami disk name
				Ebs:        ebsValue,
			},
		},
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(prefix),
					},
					{
						Key:   aws.String("Managed-By"),
						Value: aws.String("avalanche-cli"),
					},
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
func (c *AwsCloud) WaitForEC2Instances(nodeIDs []string, state types.InstanceStateName) error {
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
		allInDesiredState := true
		for _, reservation := range result.Reservations {
			for _, instance := range reservation.Instances {
				if instance.State.Name != state {
					allInDesiredState = false
					break
				}
			}
		}
		if allInDesiredState {
			return nil
		}
		// If not all instances are running, wait and retry
		time.Sleep(delay)
	}
	return fmt.Errorf("timeout waiting for instances to be in %s state", state)
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

// DestroyAWSNode terminates an EC2 instance with the given ID.
func (c *AwsCloud) DestroyAWSNode(nodeConfig models.NodeConfig, clusterName string) error {
	isRunning, err := c.checkInstanceIsRunning(nodeConfig.NodeID)
	if err != nil {
		ux.Logger.PrintToUser(fmt.Sprintf("Failed to destroy node %s due to %s", nodeConfig.NodeID, err.Error()))
		return err
	}
	if !isRunning {
		return fmt.Errorf("%w: instance %s, cluster %s", ErrNodeNotFoundToBeRunning, nodeConfig.NodeID, clusterName)
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Terminating node instance %s in cluster %s...", nodeConfig.NodeID, clusterName))
	return c.DestroyInstance(nodeConfig.NodeID, nodeConfig.ElasticIP, nodeConfig.UseStaticIP)
}

// DestroyInstance terminates an EC2 instance with the given ID.
func (c *AwsCloud) DestroyInstance(instanceID, publicIP string, releasePublicIP bool) error {
	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}
	if _, err := c.ec2Client.TerminateInstances(c.ctx, input); err != nil {
		return err
	}
	if releasePublicIP {
		if publicIP == "" {
			ux.Logger.RedXToUser("Unable to remove public IP for instance %s: undefined", instanceID)
		} else {
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
	}
	return nil
}

// CreateEIP creates an Elastic IP address.
func (c *AwsCloud) CreateEIP(prefix string) (string, string, error) {
	if addr, err := c.ec2Client.AllocateAddress(c.ctx, &ec2.AllocateAddressInput{
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeElasticIp,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(prefix),
					},
					{
						Key:   aws.String("Managed-By"),
						Value: aws.String("avalanche-cli"),
					},
				},
			},
		},
	}); err != nil {
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

// UploadSSHIdentityKeyPair uploads a key pair from ssh-agent identity to the AWS cloud.
func (c *AwsCloud) UploadSSHIdentityKeyPair(keyName string, identity string) error {
	identityValid, err := utils.IsSSHAgentIdentityValid(identity)
	if err != nil {
		return err
	}
	if !identityValid {
		return fmt.Errorf("ssh-agent identity: %s not found", identity)
	}
	publicKeyMaterial, err := utils.ReadSSHAgentIdentityPublicKey(identity)
	if err != nil {
		return err
	}
	_, err = c.ec2Client.ImportKeyPair(c.ctx, &ec2.ImportKeyPairInput{
		KeyName:           aws.String(keyName),
		PublicKeyMaterial: []byte(publicKeyMaterial),
	})
	return err
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
	if err := c.AddSecurityGroupRule(sgID, "ingress", "tcp", ipAddress, constants.AvalanchegoMonitoringPort); err != nil {
		return "", err
	}
	if err := c.AddSecurityGroupRule(sgID, "ingress", "tcp", ipAddress, constants.AvalanchegoGrafanaPort); err != nil {
		return "", err
	}
	if err := c.AddSecurityGroupRule(sgID, "ingress", "tcp", "0.0.0.0/0", constants.AvalanchegoLokiPort); err != nil {
		return "", err
	}
	if err := c.AddSecurityGroupRule(sgID, "ingress", "tcp", "0.0.0.0/0", constants.AvalanchegoP2PPort); err != nil {
		return "", err
	}
	return sgID, nil
}

// CheckIPInSg checks if the IP is present in the SecurityGroup.
func CheckIPInSg(sg *types.SecurityGroup, currentIP string, port int32) bool {
	if !strings.Contains(currentIP, "/") {
		currentIP = fmt.Sprintf("%s/32", currentIP) // add netmask /32 if missing
	}
	for _, ipPermission := range sg.IpPermissions {
		for _, ipRange := range ipPermission.IpRanges {
			cidr := *ipRange.CidrIp
			switch {
			case cidr == "0.0.0.0/0" || cidr == currentIP:
				if ipPermission.FromPort != nil && *ipPermission.FromPort == port {
					return true
				}
			default:
				_, ipNet, err := net.ParseCIDR(cidr)
				if err != nil {
					continue
				}
				ip := net.ParseIP(strings.Split(currentIP, "/")[0])
				if ip == nil {
					continue
				}
				if ipNet.Contains(ip) && ipPermission.FromPort != nil && *ipPermission.FromPort == port {
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
func (c *AwsCloud) GetUbuntuAMIID(arch string, ubuntuVerLTS string) (string, error) {
	if !utils.ArchSupported(arch) {
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
	descriptionFilterValue := fmt.Sprintf("AvalancheCLI Ubuntu %s Docker", ubuntuVerLTS)
	imageInput := &ec2.DescribeImagesInput{
		Filters: []types.Filter{
			{Name: aws.String("root-device-type"), Values: []string{"ebs"}},
			{Name: aws.String("description"), Values: []string{descriptionFilterValue}},
			{Name: aws.String("architecture"), Values: []string{arch}},
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

// ListRegions returns a list of all AWS regions.
func (c *AwsCloud) ListRegions() ([]string, error) {
	regions, err := c.ec2Client.DescribeRegions(c.ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}
	regionList := []string{}
	for _, region := range regions.Regions {
		regionList = append(regionList, *region.RegionName)
	}
	return regionList, nil
}

// isEIPQuotaExceededError checks if the error is related to exceeding the quota for Elastic IP addresses.
func isEIPQuotaExceededError(err error) bool {
	// You may need to adjust this function based on the actual error messages returned by AWS
	return err != nil && (utils.ContainsIgnoreCase(err.Error(), "limit exceeded") || utils.ContainsIgnoreCase(err.Error(), "elastic ip address limit exceeded"))
}

// GetInstanceTypeArch returns the architecture of the given instance type.
func (c *AwsCloud) GetInstanceTypeArch(instanceType string) (string, error) {
	archOutput, err := c.ec2Client.DescribeInstanceTypes(c.ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []types.InstanceType{types.InstanceType(instanceType)},
	})
	if err != nil {
		return "", err
	}
	if len(archOutput.InstanceTypes) == 0 {
		return "", fmt.Errorf("no instance type found for %s", instanceType)
	}
	return string(archOutput.InstanceTypes[0].ProcessorInfo.SupportedArchitectures[0]), nil
}

// IsInstanceTypeSupported checks if the given instance type is supported by the AWS cloud.
func (c *AwsCloud) IsInstanceTypeSupported(instanceType string) (bool, error) {
	var supportedInstanceTypes []string
	paginator := ec2.NewDescribeInstanceTypesPaginator(c.ec2Client, &ec2.DescribeInstanceTypesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(c.ctx)
		if err != nil {
			return false, err
		}

		for _, it := range output.InstanceTypes {
			supportedInstanceTypes = append(supportedInstanceTypes, string(it.InstanceType))
		}
	}
	return slices.Contains(supportedInstanceTypes, instanceType), nil
}

// GetRootVolume returns a volume IDs attached to the given which is used as a root volume
func (c *AwsCloud) GetRootVolumeID(instanceID string) (string, error) {
	describeInstanceOutput, err := c.ec2Client.DescribeInstances(c.ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return "", err
	}
	if len(describeInstanceOutput.Reservations) == 0 || len(describeInstanceOutput.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("instance with ID %s not found", instanceID)
	}
	rootDeviceName := describeInstanceOutput.Reservations[0].Instances[0].RootDeviceName

	volumeOutput, err := c.ec2Client.DescribeVolumes(c.ctx, &ec2.DescribeVolumesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("attachment.instance-id"),
				Values: []string{instanceID},
			},
			{
				Name:   aws.String("attachment.device"),
				Values: []string{*rootDeviceName},
			},
		},
	})
	if err != nil {
		return "", err
	}
	if len(volumeOutput.Volumes) == 0 {
		return "", fmt.Errorf("root volume not found for instance with ID %s", instanceID)
	}
	return *volumeOutput.Volumes[0].VolumeId, nil
}

// ResizeVolume resizes the given volume to the new size.
func (c *AwsCloud) ResizeVolume(volumeID string, newSizeInGB int32) error {
	volumeOutput, err := c.ec2Client.DescribeVolumes(c.ctx, &ec2.DescribeVolumesInput{
		VolumeIds: []string{volumeID},
	})
	if err != nil {
		return err
	}
	if volumeOutput != nil && len(volumeOutput.Volumes) == 0 {
		return fmt.Errorf("volume with ID %s not found", volumeID)
	}

	currentSize := *volumeOutput.Volumes[0].Size

	if currentSize > newSizeInGB {
		return fmt.Errorf("new size %dGb must be greater than the current size %dGb", newSizeInGB, currentSize)
	} else {
		if _, err := c.ec2Client.ModifyVolume(c.ctx, &ec2.ModifyVolumeInput{
			Size:     &newSizeInGB,
			VolumeId: volumeOutput.Volumes[0].VolumeId,
		}); err != nil {
			return err
		}
	}

	return c.WaitForVolumeModificationState(volumeID, "optimizing", 30*time.Second)
}

// WaitForVolumeModificationState waits for the specified modification state of the volume.
func (c *AwsCloud) WaitForVolumeModificationState(volumeID string, targetState string, timeout time.Duration) error {
	startTime := time.Now()
	for {
		modificationOutput, err := c.ec2Client.DescribeVolumesModifications(c.ctx, &ec2.DescribeVolumesModificationsInput{
			VolumeIds: []string{volumeID},
		})
		if err != nil {
			return err
		}
		if len(modificationOutput.VolumesModifications) == 0 {
			return fmt.Errorf("volume modification with ID %s not found", volumeID)
		}
		modificationState := modificationOutput.VolumesModifications[0].ModificationState
		if modificationState == types.VolumeModificationState(targetState) {
			break
		}
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout waiting for volume modification state to be %s", targetState)
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}

// ChangeInstanceType resizes the given instance to the new instance type.
func (c *AwsCloud) ChangeInstanceType(instanceID, instanceType string) error {
	// check if old and new instance types are the same
	resp, err := c.ec2Client.DescribeInstances(c.ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return err
	}
	if len(resp.Reservations) == 0 || len(resp.Reservations[0].Instances) == 0 {
		return fmt.Errorf("instance not found")
	}
	currentInstanceType := resp.Reservations[0].Instances[0].InstanceType
	if currentInstanceType == types.InstanceType(instanceType) {
		return fmt.Errorf("instance %s is already of type %s", instanceID, instanceType)
	}

	// stop the instance
	if _, err := c.ec2Client.StopInstances(c.ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	}); err != nil {
		return err
	}
	if err := c.WaitForEC2Instances([]string{instanceID}, types.InstanceStateNameStopped); err != nil {
		return err
	}
	// update the instance type
	if _, err := c.ec2Client.ModifyInstanceAttribute(c.ctx, &ec2.ModifyInstanceAttributeInput{
		InstanceId: aws.String(instanceID),
		InstanceType: &types.AttributeValue{
			Value: aws.String(instanceType),
		},
	}); err != nil {
		return err
	}
	// start the instance
	if _, err := c.ec2Client.StartInstances(c.ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	}); err != nil {
		return err
	}
	return nil
}
