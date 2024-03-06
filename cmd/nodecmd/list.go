// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/models"
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
	for clusterName, clusterConf := range clustersConfig.Clusters {
		ux.Logger.PrintToUser("Cluster %q (%s)", clusterName, clusterConf.Network.Name())
		if err := checkCluster(clusterName); err != nil {
			return err
		}
		ansibleHostIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
		if err != nil {
			return err
		}
		for _, ansibleHostID := range ansibleHostIDs {
			_, cloudHostID, err := models.HostAnsibleIDToCloudID(ansibleHostID)
			if err != nil {
				return err
			}
			nodeConfig, err := app.LoadClusterNodeConfig(cloudHostID)
			if err != nil {
				return err
			}
			nodeIDStr := "----------------------------------------"
			funcs := []string{}
			if clusterConf.MonitoringInstance != cloudHostID {
				nodeID, err := getNodeID(app.GetNodeInstanceDirPath(cloudHostID))
				if err != nil {
					return err
				}
				nodeIDStr = nodeID.String()
				if clusterConf.IsAPIHost(cloudHostID) {
					funcs = append(funcs, "API")
				} else {
					funcs = append(funcs, "Node")
				}
			}
			if nodeConfig.IsMonitor {
				funcs = append(funcs, "Monitor")
			}
			funcDesc := strings.Join(funcs, ",")
			if funcDesc != "" {
				funcDesc = " [" + funcDesc + "]"
			}
			ux.Logger.PrintToUser(fmt.Sprintf("  Node %s (%s) %s%s", cloudHostID, nodeIDStr, nodeConfig.ElasticIP, funcDesc))
		}
	}
	return nil
}
