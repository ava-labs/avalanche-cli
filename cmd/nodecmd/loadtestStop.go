// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/aws"
	gcpAPI "github.com/ava-labs/avalanche-cli/pkg/cloud/gcp"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newLoadTestStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [clusterName]",
		Short: "(ALPHA Warning) Stops load test for an existing devnet cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode. 

The node loadtest stop command stops load testing for an existing devnet cluster and terminates the 
separate cloud server created to host the load test.`,

		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         stopLoadTest,
	}
	return cmd
}

func stopLoadTest(_ *cobra.Command, args []string) error {
	loadTestName := args[0]
	clusterName := args[1]
	var err error
	existingSeparateInstance, err = getExistingLoadTestInstance(clusterName, loadTestName)
	if err != nil {
		return err
	}
	if existingSeparateInstance == "" {
		return fmt.Errorf("no existing load test instance found in cluster %s", clusterName)
	}
	nodeConfig, err := app.LoadClusterNodeConfig(existingSeparateInstance)
	switch nodeConfig.CloudService {
	case constants.AWSCloudService:
		loadTestEc2SvcMap := make(map[string]*awsAPI.AwsCloud)
		_, separateHostRegion, err := getNodeCloudConfig(existingSeparateInstance)
		if err != nil {
			return err
		}
		loadTestEc2SvcMap, err = getAWSMonitoringEC2Svc(awsProfile, separateHostRegion)
		if err != nil {
			return err
		}
		if err = destroyNode(existingSeparateInstance, clusterName, loadTestName, loadTestEc2SvcMap[separateHostRegion], nil); err != nil {
			return err
		}
	case constants.GCPCloudService:
		var gcpClient *gcpAPI.GcpCloud
		gcpClient, _, _, _, _, err = getGCPConfig(true)
		if err != nil {
			return err
		}
		if err = destroyNode(existingSeparateInstance, clusterName, loadTestName, nil, gcpClient); err != nil {
			return err
		}
	default:
		return fmt.Errorf("cloud service %s is not supported", nodeConfig.CloudService)
	}
	return nil
}

func destroyNode(node, clusterName, loadTestName string, ec2Svc *awsAPI.AwsCloud, gcpClient *gcpAPI.GcpCloud) error {
	nodeConfig, err := app.LoadClusterNodeConfig(node)
	if err != nil {
		ux.Logger.PrintToUser("Failed to destroy node %s", node)
		return err
	}
	if nodeConfig.CloudService == "" || nodeConfig.CloudService == constants.AWSCloudService {
		if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.AWSCloudService) != nil) {
			return fmt.Errorf("cloud access is required")
		}
		if err = ec2Svc.DestroyAWSNode(nodeConfig, ""); err != nil {
			if isExpiredCredentialError(err) {
				ux.Logger.PrintToUser("")
				printExpiredCredentialsOutput(awsProfile)
				return nil
			}
			if !errors.Is(err, awsAPI.ErrNodeNotFoundToBeRunning) {
				return err
			}
			ux.Logger.PrintToUser("node %s is already destroyed", nodeConfig.NodeID)
		}
	} else {
		if !(authorizeAccess || authorizedAccessFromSettings()) && (requestCloudAuth(constants.GCPCloudService) != nil) {
			return fmt.Errorf("cloud access is required")
		}
		if err = gcpClient.DestroyGCPNode(nodeConfig, ""); err != nil {
			if !errors.Is(err, gcpAPI.ErrNodeNotFoundToBeRunning) {
				return err
			}
			ux.Logger.PrintToUser("node %s is already destroyed", nodeConfig.NodeID)
		}
	}
	ux.Logger.PrintToUser("Node instance %s successfully destroyed!", nodeConfig.NodeID)
	if err := removeDeletedNodeDirectory(node); err != nil {
		ux.Logger.PrintToUser("Failed to delete node config for node %s due to %s", node, err.Error())
		return err
	}
	if err := removeLoadTestNodeFromClustersConfig(clusterName, loadTestName); err != nil {
		ux.Logger.PrintToUser("Failed to delete node config for node %s due to %s", node, err.Error())
		return err
	}
	return removeLoadTestInventoryDir(clusterName)
}

func removeLoadTestNodeFromClustersConfig(clusterName, loadTestName string) error {
	clustersConfig := models.ClustersConfig{}
	var err error
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if clustersConfig.Clusters != nil {
		if _, ok := clustersConfig.Clusters[clusterName]; !ok {
			return fmt.Errorf("cluster %s is not found in cluster config", clusterName)
		}
		clusterConfig := clustersConfig.Clusters[clusterName]
		if _, ok := clusterConfig.LoadTestInstance[loadTestName]; ok {
			if clustersConfig.Clusters != nil {
				delete(clusterConfig.LoadTestInstance, loadTestName)
			}
		}
	}
	return app.WriteClustersConfigFile(&clustersConfig)
}

func removeLoadTestInventoryDir(clusterName string) error {
	return os.RemoveAll(app.GetLoadTestInventoryDir(clusterName))
}
