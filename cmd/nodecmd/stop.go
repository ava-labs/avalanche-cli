// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"
	"os"

	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [clusterName]",
		Short: "(ALPHA Warning) Stop all nodes in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node stop command stops a running node in cloud server

Note that a stopped node may still incur cloud server storage fees.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         stopNode,
	}

	return cmd
}

func removeNodeFromClusterConfig(clusterName string) error {
	clusterConfig := models.ClusterConfig{}
	var err error
	if app.ClusterConfigExists() {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return err
		}
	}
	if clusterConfig.Clusters != nil {
		delete(clusterConfig.Clusters, clusterName)
	}
	return app.WriteClusterConfigFile(&clusterConfig)
}

func removeDeletedNodeDirectory(clusterName string) error {
	return os.RemoveAll(app.GetNodeInstanceDirPath(clusterName))
}

func removeClusterInventoryDir(clusterName string) error {
	return os.RemoveAll(app.GetAnsibleInventoryDirPath(clusterName))
}

func getDeleteConfigConfirmation() error {
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

func removeClusterConfigFiles(clusterName string) error {
	if err := removeClusterInventoryDir(clusterName); err != nil {
		return err
	}
	return removeNodeFromClusterConfig(clusterName)
}

func stopNode(_ *cobra.Command, args []string) error {
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
	failedNodes := []string{}
	nodeErrors := []error{}
	lastRegion := ""
	var ec2Svc *ec2.EC2
	for _, node := range clusterNodes {
		nodeConfig, err := app.LoadClusterNodeConfig(node)
		if err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, err.Error()))
			failedNodes = append(failedNodes, node)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		if nodeConfig.Region != lastRegion {
			sess, err := getAWSCloudCredentials(nodeConfig.Region, true)
			if err != nil {
				return err
			}
			ec2Svc = ec2.New(sess)
			lastRegion = nodeConfig.Region
		}
		isRunning, err := awsAPI.CheckInstanceIsRunning(ec2Svc, nodeConfig.NodeID)
		if err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, err.Error()))
			failedNodes = append(failedNodes, node)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		if !isRunning {
			noRunningNodeErr := fmt.Errorf("no running node with instance id %s is found in cluster %s", nodeConfig.NodeID, clusterName)
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, noRunningNodeErr))
			failedNodes = append(failedNodes, node)
			nodeErrors = append(nodeErrors, noRunningNodeErr)
			continue
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Stopping node instance %s in cluster %s...", nodeConfig.NodeID, clusterName))
		if err := awsAPI.StopInstance(ec2Svc, nodeConfig.NodeID, nodeConfig.ElasticIP, true); err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, err.Error()))
			failedNodes = append(failedNodes, node)
			nodeErrors = append(nodeErrors, err)
			continue
		}
		ux.Logger.PrintToUser(fmt.Sprintf("Node instance %s in cluster %s successfully stopped!", nodeConfig.NodeID, clusterName))
		if err := removeDeletedNodeDirectory(node); err != nil {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to delete node config for node %s due to %s", node, err.Error()))
			return err
		}
	}
	if len(failedNodes) > 0 {
		ux.Logger.PrintToUser("Failed nodes: ")
		for i, node := range failedNodes {
			ux.Logger.PrintToUser(fmt.Sprintf("Failed to stop node %s due to %s", node, nodeErrors[i]))
		}
		return fmt.Errorf("failed to stop node(s) %s", failedNodes)
	} else {
		ux.Logger.PrintToUser(fmt.Sprintf("All nodes in cluster %s are successfully stopped!", clusterName))
	}
	return removeClusterConfigFiles(clusterName)
}

func checkCluster(clusterName string) error {
	_, err := getClusterNodes(clusterName)
	return err
}

func getClusterNodes(clusterName string) ([]string, error) {
	clusterConfig := models.ClusterConfig{}
	if app.ClusterConfigExists() {
		var err error
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	if _, ok := clusterConfig.Clusters[clusterName]; !ok {
		return nil, fmt.Errorf("cluster %q does not exist", clusterName)
	}
	clusterNodes := clusterConfig.Clusters[clusterName]
	if len(clusterNodes) == 0 {
		return nil, fmt.Errorf("no nodes found in cluster %s", clusterName)
	}
	return clusterNodes, nil
}
