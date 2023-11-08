// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
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
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return err
		}
	}
	if len(clustersConfig.Clusters) == 0 {
		ux.Logger.PrintToUser("There are no clusters defined.")
	}
	for clusterName, clusterConfig := range clustersConfig.Clusters {
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
		for _, clusterNode := range clusterConfig.Nodes {
			nodeConfig, err := app.LoadClusterNodeConfig(clusterNode)
			if err != nil {
				return err
			}
			hostName, err := models.HostCloudIDToAnsibleID(nodeConfig.CloudService, clusterNode)
			if err != nil {
				return err
			}
			nodeID, err := getNodeID(app.GetNodeInstanceDirPath(clusterNode))
			if err != nil {
				return err
			}
			ux.Logger.PrintToUser(fmt.Sprintf("  Node %s", clusterNode))
			ux.Logger.PrintToUser(fmt.Sprintf("    Avalanche ID: %s", nodeID.String()))
			ux.Logger.PrintToUser(fmt.Sprintf("    SSH cmd: %s", utils.GetSSHConnectionString(ansibleHosts[hostName].IP, ansibleHosts[hostName].SSHPrivateKeyPath)))
		}
	}
	return nil
}
