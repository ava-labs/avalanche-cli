// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
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
change amount of CPU, memory and disk space avalable for the cluster nodes.
`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE:         resize,
	}
	cmd.Flags().StringVar(&nodeType, "node-type", "", "Node type to resize (e.g. t3.2xlarge)")
	cmd.Flags().StringVar(&diskSize, "disk-size", "", "Disk size to resize in Gb (e.g. 100Gb)")
	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")
	return cmd
}

func preResizeChecks() error {
	if nodeType == "" && diskSize == "" {
		return fmt.Errorf("at least one of the flags --node-type or --disk-size must be provided")
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
				ux.Logger.RedXToUser("Failed to resize node size %s: %v", host.GetCloudID(), err)
			}
		}
		if diskSize != "" {
			if err := resizeDisk(nodeConfig, diskSize); err != nil {
				ux.Logger.RedXToUser("Failed to resize disk size %s: %v", host.GetCloudID(), err)
			}
			if err := ssh.RunSSHUpsizeDisk(nodeConfig, diskSize); err != nil {
				ux.Logger.RedXToUser("Failed to resize disk size %s: %v", host.GetCloudID(), err)
			}
		}
	}
	return nil
}

func resizeDisk(nodeConfig models.NodeConfig, diskSize int) error {
	switch nodeConfig.CloudService {
	case "", constants.AWSCloudService:
		ec2Svc, err := awsAPI.NewAwsCloud(awsProfile, nodeConfig.Region)
		if err != nil {
			return err
		}
		volumes, err := ec2Svc.ListAttachedVolumes(nodeConfig.NodeID)
		if err != nil {
			return err
		}
		if len(volumes) != 1 {
			return fmt.Errorf("expected 1 volume attached to instance, got %d", len(volumes))
		}
		return ec2Svc.ResizeVolume(volumes[0], int32(diskSize))
	case constants.GCPCloudService:
		gcpClient, projectName, _, err := getGCPCloudCredentials()
		if err != nil {
			return err
		}
		gcpCloud, err := gcpAPI.NewGcpCloud(gcpClient, projectName, context.Background())
		if err != nil {
			return err
		}
		volumes, err := gcpCloud.ListAttachedVolumes(nodeConfig.NodeID, nodeConfig.Region)
		if err != nil {
			return err
		}
		if len(volumes) != 1 {
			return fmt.Errorf("expected 1 volume attached to instance, got %d", len(volumes))
		}
		return gcpCloud.ResizeVolume(volumes[0], nodeConfig.Region, int64(diskSize))
	default:
		return fmt.Errorf("cloud service %s is not supported", nodeConfig.CloudService)
	}
	return nil
}
