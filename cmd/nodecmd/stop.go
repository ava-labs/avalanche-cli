// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [clusterName]",
		Short: "(ALPHA Warning) Stop all nodes in a cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node stop command stops a running node in cloud server`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         stopNode,
	}

	return cmd
}

func stopNode(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	//if err := setupAnsible(); err != nil {
	//	return err
	//}
	//nodeIDStr, err := getNodeID(clusterName)
	//if err != nil {
	//	return err
	//}
	var err error
	clusterConfig := models.ClusterConfig{}
	if app.ClusterConfigExists() {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return err
		}
	}
	clusterNodes := clusterConfig.Clusters[clusterName]
	if len(clusterNodes) == 0 {
		return fmt.Errorf("no nodes found in cluster %s", clusterName)
	}
	fmt.Printf("obtained node id %s \n", clusterNodes[0])
	nodeConfig, err := app.LoadClusterNodeConfig(clusterNodes[0])
	if err != nil {
		return err
	}
	//region := "us-east-2"
	fmt.Printf("obtained region %s \n", nodeConfig.Region)
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
	return nil
}
