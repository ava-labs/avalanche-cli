// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "(ALPHA Warning) List all clusters together with their nodes",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node list command lists all clusters together with their nodes.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE:         list,
	}

	return cmd
}

func list(_ *cobra.Command, _ []string) error {
	var err error
	clusterConfig := models.ClusterConfig{}
	if app.ClusterConfigExists() {
		clusterConfig, err = app.LoadClusterConfig()
		if err != nil {
			return err
		}
	}
	for clusterName, clusterNodes := range clusterConfig.Clusters {
		ux.Logger.PrintToUser(fmt.Sprintf("Cluster %q", clusterName))
		if err := checkCluster(clusterName); err != nil {
			return err
		}
		if err := setupAnsible(clusterName); err != nil {
			return err
		}
		ansibleHosts, err := ansible.GetHostMapfromAnsibleInventory(app.GetAnsibleInventoryDirPath(clusterName))
		if err != nil {
			return err
		}
		for _, clusterNode := range clusterNodes {
			ux.Logger.PrintToUser(fmt.Sprintf("  Node %q to connect: %s", clusterNode, utils.GetSSHConnectionString(ansibleHosts[constants.AnsibleAWSNodePrefix+clusterNode].SSHCommonArgs, ansibleHosts[constants.AnsibleAWSNodePrefix+clusterNode].IP, ansibleHosts[constants.AnsibleAWSNodePrefix+clusterNode].SSHPrivateKeyPath)))
		}
	}
	return nil
}
