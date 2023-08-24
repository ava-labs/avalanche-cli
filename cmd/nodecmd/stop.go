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
	return os.RemoveAll(app.GetAnsibleInventoryPath(clusterName))
}

func getDeleteConfigConfirmation(instanceID string) error {
	confirm := "Running this command will delete all stored files associated with your cloud server. Do you want to proceed? " +
		fmt.Sprintf("Stored files can be found at %s", app.GetNodeInstanceDirPath(instanceID))
	yes, err := app.Prompt.CaptureYesNo(confirm)
	if err != nil {
		return err
	}
	if !yes {
		return errors.New("abort avalanche stop node command")
	}
	return nil
}

func removeConfigFiles(clusterName string) error {
	if err := removeDeletedNodeDirectory(clusterName); err != nil {
		return err
	}
	if err := removeClusterInventoryDir(clusterName); err != nil {
		return err
	}
	return removeNodeFromClusterConfig(clusterName)
}

func stopNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	var err error
	clusterConfig := models.ClusterConfig{}
	if app.ClusterConfigExists() {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return err
		}
	}
	if _, ok := clusterConfig.Clusters[clusterName]; !ok {
		return fmt.Errorf("unable to find state file for cluster %s in .avalanche-cli dir", clusterName)
	}
	clusterNodes := clusterConfig.Clusters[clusterName]
	if len(clusterNodes) == 0 {
		return fmt.Errorf("no nodes found in cluster %s", clusterName)
	}
	nodeConfig, err := app.LoadClusterNodeConfig(clusterNodes[0])
	if err != nil {
		return err
	}
	if err = getDeleteConfigConfirmation(nodeConfig.NodeID); err != nil {
		return err
	}
	sess, err := getAWSCloudCredentials(nodeConfig.Region)
	if err != nil {
		return err
	}
	ec2Svc := ec2.New(sess)
	isRunning, err := awsAPI.CheckInstanceIsRunning(ec2Svc, nodeConfig.NodeID)
	if err != nil {
		return err
	}
	if !isRunning {
		return fmt.Errorf("no running node with instance id %s is found in cluster %s", nodeConfig.NodeID, clusterName)
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Stopping node instance %s in cluster %s...", nodeConfig.NodeID, clusterName))
	if err = awsAPI.StopInstance(ec2Svc, nodeConfig.NodeID, nodeConfig.ElasticIP); err != nil {
		return err
	}
	if err = removeConfigFiles(clusterName); err != nil {
		return err
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Node instance %s in cluster %s successfully stopped!", nodeConfig.NodeID, clusterName))
	return nil
}
