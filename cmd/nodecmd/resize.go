// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

var diskSize string

func newResizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resize [clusterName]",
		Short: "(ALPHA Warning) Resize cluster node and disk sizes",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node resize command can be used to resize cluster instance size 
and/or size of the permanent storage attached to the instance. In another words, it can 
change amount of CPU, memory and disk space available for the cluster nodes.
`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE:         resize,
	}
	cmd.Flags().StringVar(&nodeType, "node-type", "", "Node type to resize (e.g. t3.2xlarge)")
	cmd.Flags().StringVar(&diskSize, "disk-size", "", "Disk size to resize in Gb (e.g. 1000Gi)")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	return cmd
}

func preResizeChecks() error {
	if nodeType == "" && diskSize == "" {
		return fmt.Errorf("at least one of the flags --node-type or --disk-size must be provided")
	}
	if !strings.HasSuffix(diskSize, "Gi") {
		return fmt.Errorf("disk-size must be in Gi")
	}
	diskSizeGi := strings.TrimSuffix(diskSize, "Gi")
	if _, err := strconv.Atoi(diskSizeGi); err != nil {
		return fmt.Errorf("disk-size must be an integer")
	}
	return nil
}

func resize(_ *cobra.Command, args []string) error {
	if err := preResizeChecks(); err != nil {
		return err
	}
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	clusterNodes, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	monitoringNode, err := getClusterMonitoringNode(clusterName)
	if err != nil {
		return err
	}
	nodesToResize := utils.Filter(clusterNodes, func(node string) bool {
		return node != monitoringNode
	})

	for _, node := range nodesToResize {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			return err
		}
		hostAnsibleID, err := models.HostCloudIDToAnsibleID(nodeConfig.CloudService, nodeConfig.NodeID)
		if err != nil {
			return err
		}
		host, err := ansible.GetHostByNodeID(hostAnsibleID, app.GetAnsibleInventoryDirPath(clusterName))
		if err != nil {
			return err
		}
		if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(nodeConfig.CloudService) != nil) {
			return fmt.Errorf("cloud access is required")
		}
		// resize node and disk. If error occurs, log it and continue to next host
		if nodeType != "" {
			if err := resizeNode(nodeConfig); err != nil {
				if isExpiredCredentialError(err) {
					ux.Logger.PrintToUser("")
					printExpiredCredentialsOutput(awsProfile)
					return nil
				}
				ux.Logger.RedXToUser("Failed to resize node size %s: %v", nodeConfig.NodeID, err)
			}
		}
		if diskSize != "" {
			diskSizeGi, _ := strconv.Atoi(strings.TrimSuffix(diskSize, "Gi"))
			if err := resizeDisk(nodeConfig, diskSizeGi); err != nil {
				ux.Logger.RedXToUser("Failed to resize disk size %s: %v", nodeConfig.NodeID, err)
			} else if err := ssh.RunSSHUpsizeRootDisk(host); err != nil {
				ux.Logger.RedXToUser("Failed to resize root disk on node %s: %v", nodeConfig.NodeID, err)
			} else {
				ux.Logger.GreenCheckmarkToUser("Successfully resized disk size %s", nodeConfig.NodeID)
			}
		}
	}
	return nil
}

// resizeDisk resizes the disk size of the node
func resizeDisk(nodeConfig models.NodeConfig, diskSize int) error {
	switch nodeConfig.CloudService {
	case "", constants.AWSCloudService:
		ec2Svc, err := awsAPI.NewAwsCloud(awsProfile, nodeConfig.Region)
		if err != nil {
			return err
		}
		rootVolume, err := ec2Svc.GetRootVolumeID(nodeConfig.NodeID)
		if err != nil {
			return err
		}
		return ec2Svc.ResizeVolume(rootVolume, int32(diskSize))
	case constants.GCPCloudService:
		gcpClient, projectName, _, err := getGCPCloudCredentials()
		if err != nil {
			return err
		}
		gcpCloud, err := gcpAPI.NewGcpCloud(gcpClient, projectName, context.Background())
		if err != nil {
			return err
		}
		rootVolume, err := gcpCloud.GetRootVolumeID(nodeConfig.NodeID, nodeConfig.Region)
		if err != nil {
			return err
		}
		return gcpCloud.ResizeVolume(rootVolume, nodeConfig.Region, int64(diskSize))
	default:
		return fmt.Errorf("cloud service %s is not supported", nodeConfig.CloudService)
	}
}

// resizeNode changes the node type of the instance
func resizeNode(nodeConfig models.NodeConfig) error {
	switch nodeConfig.CloudService {
	case "", constants.AWSCloudService:
		ec2Svc, err := awsAPI.NewAwsCloud(awsProfile, nodeConfig.Region)
		if err != nil {
			return err
		}
		if isSupported, err := ec2Svc.IsInstanceTypeSupported(nodeType); err != nil || !isSupported {
			return fmt.Errorf("instance type %s is not supported with err: %w", nodeType, err)
		}
		return ec2Svc.ChangeInstanceType(nodeConfig.NodeID, nodeType)
	case constants.GCPCloudService:
		gcpClient, projectName, _, err := getGCPCloudCredentials()
		if err != nil {
			return err
		}
		gcpCloud, err := gcpAPI.NewGcpCloud(gcpClient, projectName, context.Background())
		if err != nil {
			return err
		}
		if isSupported, err := gcpCloud.IsInstanceTypeSupported(nodeType, nodeConfig.Region); err != nil || !isSupported {
			return fmt.Errorf("instance type %s is not supported with err: %w", nodeType, err)
		}
		return gcpCloud.ChangeInstanceType(nodeConfig.NodeID, nodeConfig.Region, nodeType)
	default:
		return fmt.Errorf("cloud service %s is not supported", nodeConfig.CloudService)
	}
}
