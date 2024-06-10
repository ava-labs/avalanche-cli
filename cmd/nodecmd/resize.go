// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
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

The node resize command can change the amount of CPU, memory and disk space available for the cluster nodes.
`,
		Args: cobrautils.MinimumNArgs(1),
		RunE: resize,
	}
	cmd.Flags().StringVar(&nodeType, "node-type", "", "Node type to resize (e.g. t3.2xlarge)")
	cmd.Flags().StringVar(&diskSize, "disk-size", "", "Disk size to resize in Gb (e.g. 1000Gb)")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	return cmd
}

func preResizeChecks(clusterName string) error {
	if nodeType == "" && diskSize == "" {
		return fmt.Errorf("at least one of the flags --node-type or --disk-size must be provided")
	}
	if diskSize != "" && !strings.HasSuffix(diskSize, "Gb") {
		return fmt.Errorf("disk-size must be in Gb")
	}
	if diskSize != "" {
		diskSizeGb := strings.TrimSuffix(diskSize, "Gb")
		if _, err := strconv.Atoi(diskSizeGb); err != nil {
			return fmt.Errorf("disk-size must be an integer")
		}
	}
	if err := failForExternal(clusterName); err != nil {
		return fmt.Errorf("cannot resize external cluster %s", clusterName)
	}
	return nil
}

func resize(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if err := preResizeChecks(clusterName); err != nil {
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

	if nodeType != "" {
		ux.Logger.PrintLineSeparator()
		ux.Logger.PrintToUser("Node performance may be impacted during resizing")
		ux.Logger.PrintToUser("Please note that instances will be restarted during resizing.")
		ux.Logger.PrintToUser("This operation may take some time to complete. Thank you for your patience.")
	}

	if diskSize != "" {
		ux.Logger.PrintLineSeparator()
		ux.Logger.PrintToUser("Disk performance may be impacted during resizing")
		ux.Logger.PrintToUser("Please ensure that the cluster is not under heavy load.")
	}

	for _, node := range nodesToResize {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			return err
		}
		hostAnsibleID, err := ansible.HostCloudIDToAnsibleID(nodeConfig.CloudService, nodeConfig.NodeID)
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
		spinSession := ux.NewUserSpinner()
		// resize node and disk. If error occurs, log it and continue to next host
		if nodeType != "" {
			spinner := spinSession.SpinToUser(utils.ScriptLog(nodeConfig.NodeID, "Resizing Instance Type"))
			if err := resizeNode(nodeConfig); err != nil {
				ux.SpinFailWithError(spinner, "", err)
			} else {
				ux.SpinComplete(spinner)
			}
		}
		if diskSize != "" {
			spinner := spinSession.SpinToUser(utils.ScriptLog(nodeConfig.NodeID, "Resizing Disk"))
			diskSizeGb, _ := strconv.Atoi(strings.TrimSuffix(diskSize, "Gb"))
			if err := resizeDisk(nodeConfig, diskSizeGb); err != nil {
				ux.SpinFailWithError(spinner, "", err)
			} else if err := ssh.RunSSHUpsizeRootDisk(host); err != nil {
				ux.SpinFailWithError(spinner, "", err)
			} else {
				ux.SpinComplete(spinner)
			}
		}
		spinSession.Stop()
	}
	return nil
}

// resizeDisk resizes the disk size of the node
func resizeDisk(nodeConfig models.NodeConfig, diskSize int) error {
	if diskSize > math.MaxInt32 {
		return fmt.Errorf("disk size exceeds maximum supported value")
	}
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
		if diskSize > math.MaxInt {
			return fmt.Errorf("disk size exceeds maximum supported value")
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
		isSupported, err := ec2Svc.IsInstanceTypeSupported(nodeType)
		if err != nil {
			return err
		}
		if !isSupported {
			return fmt.Errorf("instance type %s is not supported", nodeType)
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
		isSupported, err := gcpCloud.IsInstanceTypeSupported(nodeType, nodeConfig.Region)
		if err != nil {
			return err
		}
		if !isSupported {
			return fmt.Errorf("instance type %s is not supported", nodeType)
		}
		return gcpCloud.ChangeInstanceType(nodeConfig.NodeID, nodeConfig.Region, nodeType)
	default:
		return fmt.Errorf("cloud service %s is not supported", nodeConfig.CloudService)
	}
}
