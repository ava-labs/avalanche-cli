// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/node"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/spf13/cobra"
)

func newRefreshIPsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh-ips [clusterName]",
		Short: "(ALPHA Warning) Refresh IPs for nodes with dynamic IPs in the cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node refresh-ips command obtains the current IP for all nodes with dynamic IPs in the cluster,
and updates the local node information used by CLI commands.`,
		Args: cobrautils.ExactArgs(1),
		RunE: refreshIPs,
	}

	cmd.Flags().StringVar(&awsProfile, "aws-profile", constants.AWSDefaultCredential, "aws profile to use")

	return cmd
}

func refreshIPs(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := node.CheckCluster(app, clusterName); err != nil {
		return err
	}
	if err := failForExternal(clusterName); err != nil {
		return err
	}
	return updatePublicIPs(clusterName)
}

func failForExternal(clusterName string) error {
	external, err := checkClusterExternal(clusterName)
	if err != nil {
		return err
	}
	if external {
		return fmt.Errorf("cannot refresh IPs for external cluster %s", clusterName)
	}
	return nil
}
