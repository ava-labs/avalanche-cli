// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"golang.org/x/exp/maps"
	"golang.org/x/net/context"

	"github.com/ava-labs/avalanche-cli/pkg/constants"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var authorizeRemove bool

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [clusterName]",
		Short: "(ALPHA Warning) Stop all nodes in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node stop command stops a running node in cloud server

Note that a stopped node may still incur cloud server storage fees.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         stopNodes,
	}
	cmd.Flags().BoolVar(&authorizeAccess, "authorize-access", false, "authorize CLI to release cloud resources")
	cmd.Flags().BoolVar(&authorizeRemove, "authorize-remove", false, "authorize CLI to remove all local files related to cloud nodes")

	return cmd
}

func removeNodeFromClustersConfig(clusterName string) error {
	clustersConfig := models.ClustersConfig{}
	var err error
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if clustersConfig.Clusters != nil {
		delete(clustersConfig.Clusters, clusterName)
	}
	return app.WriteClustersConfigFile(&clustersConfig)
}

func removeDeletedNodeDirectory(clusterName string) error {
	return os.RemoveAll(app.GetNodeInstanceDirPath(clusterName))
}

func removeClusterInventoryDir(clusterName string) error {
	return os.RemoveAll(app.GetAnsibleInventoryDirPath(clusterName))
}

func getDeleteConfigConfirmation() error {
	if authorizeRemove {
		return nil
	}
	ux.Logger.PrintToUser("Please note that if your node(s) are validating a Subnet, stopping them could cause Subnet instability and it is irreversible")
	confirm := "Running this command will delete all stored files associated with your cloud server. Do you want to proceed? " +
		fmt.Sprintf("Stored files can be found at %s", app.GetNodesDir())
	yes, err := app.Prompt.CaptureYesNo(confirm)
	if err != nil {
		return err
	}
	if !yes {
		return errors.New("abort avalanche stop node command")
	}
	return nil
}

func removeClustersConfigFiles(clusterName string) error {
	if err := removeClusterInventoryDir(clusterName); err != nil {
		return err
	}
	return removeNodeFromClustersConfig(clusterName)
}

func stopNodes(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if err := getDeleteConfigConfirmation(); err != nil {
		return err
	}
	clusterNodes, err := getClusterNodes(clusterName)
	if err != nil {
		return err
	}
	nodeErrors := map[string]error{}
	lastRegion := ""
	var ec2Svc *awsAPI.AwsCloud
	var gcpCloud *gcpAPI.GcpCloud
	for _, node := range clusterNodes {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			nodeErrors[node] = err
			ux.Logger.PrintToUser("Failed to stop node %s due to %s", node, err.Error())
			continue
		}
		if nodeConfig.CloudService == "" || nodeConfig.CloudService == constants.AWSCloudService {
			if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.AWSCloudService) != nil) {
				return fmt.Errorf("cloud access is required")
			}
			// need to check if it's empty because we didn't set cloud service when only using AWS
			if nodeConfig.Region != lastRegion {
				ec2Svc, err = awsAPI.NewAwsCloud(awsProfile, nodeConfig.Region)
				if err != nil {
					return err
				}
				lastRegion = nodeConfig.Region
			}
			if err = ec2Svc.StopAWSNode(nodeConfig, clusterName); err != nil {
				if strings.Contains(err.Error(), "RequestExpired: Request has expired") {
					ux.Logger.PrintToUser("")
					printExpiredCredentialsOutput(awsProfile)
					return nil
				}
				if !errors.Is(err, awsAPI.ErrNodeNotFoundToBeRunning) {
					nodeErrors[node] = err
					continue
				}
				ux.Logger.PrintToUser("node %s is already stopped", nodeConfig.NodeID)
			}
		} else {
			if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.GCPCloudService) != nil) {
				return fmt.Errorf("cloud access is required")
			}
			if gcpCloud == nil {
				gcpClient, projectName, _, err := getGCPCloudCredentials()
				if err != nil {
					return err
				}
				gcpCloud, err = gcpAPI.NewGcpCloud(gcpClient, projectName, context.Background())
				if err != nil {
					return err
				}
			}
			if err = gcpCloud.StopGCPNode(nodeConfig, clusterName); err != nil {
				if !errors.Is(err, gcpAPI.ErrNodeNotFoundToBeRunning) {
					nodeErrors[node] = err
					continue
				}
				ux.Logger.PrintToUser("node %s is already stopped", nodeConfig.NodeID)
			}
		}
		ux.Logger.PrintToUser("Node instance %s in cluster %s successfully stopped!", nodeConfig.NodeID, clusterName)
		if err := removeDeletedNodeDirectory(node); err != nil {
			ux.Logger.PrintToUser("Failed to delete node config for node %s due to %s", node, err.Error())
			return err
		}
	}
	if len(nodeErrors) > 0 {
		ux.Logger.PrintToUser("Failed nodes: ")
		for node, nodeErr := range nodeErrors {
			if strings.Contains(nodeErr.Error(), constants.ErrReleasingGCPStaticIP) {
				ux.Logger.PrintToUser("Node is stopped, but failed to release static ip address for node %s due to %s", node, nodeErr)
			} else {
				ux.Logger.PrintToUser("Failed to stop node %s due to %s", node, nodeErr)
			}
		}
		return fmt.Errorf("failed to stop node(s) %s", maps.Keys(nodeErrors))
	} else {
		ux.Logger.PrintToUser("All nodes in cluster %s are successfully stopped!", clusterName)
	}
	return removeClustersConfigFiles(clusterName)
}

func checkCluster(clusterName string) error {
	_, err := getClusterNodes(clusterName)
	return err
}

func getClusterNodes(clusterName string) ([]string, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return nil, err
		}
	}
	if _, ok := clustersConfig.Clusters[clusterName]; !ok {
		return nil, fmt.Errorf("cluster %q does not exist", clusterName)
	}
	clusterNodes := clustersConfig.Clusters[clusterName].Nodes
	if len(clusterNodes) == 0 {
		return nil, fmt.Errorf("no nodes found in cluster %s", clusterName)
	}
	return clusterNodes, nil
}

func clusterExists(clusterName string) (bool, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return false, err
		}
	}
	_, ok := clustersConfig.Clusters[clusterName]
	return ok, nil
}
