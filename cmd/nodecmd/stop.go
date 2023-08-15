// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	awsAPI "github.com/ava-labs/avalanche-cli/pkg/aws"
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
	err := setupAnsible()
	if err != nil {
		return err
	}
	nodeIDStr, err := getNodeID(clusterName)
	if err != nil {
		return err
	}
	region := "us-east-2"
	sess, err := getAWSCloudCredentials(region)
	if err != nil {
		return err
	}
	ec2Svc := ec2.New(sess)
	_, err = awsAPI.GetInstanceStatus(ec2Svc, nodeIDStr)
	if err != nil {
		return err
	}
	return nil
}
