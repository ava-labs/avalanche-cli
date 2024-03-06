// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"path/filepath"
	"strings"

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
		ansibleHosts, err := ansible.GetHostMapfromAnsibleInventory(app.GetAnsibleInventoryDirPath(clusterName))
		if err != nil {
			return err
		}

		var monitoringAnsibleHosts map[string]*models.Host
		monitoringInventoryPath := filepath.Join(app.GetAnsibleInventoryDirPath(clusterName), constants.MonitoringDir)
		if utils.DirectoryExists(monitoringInventoryPath) {
			monitoringAnsibleHostIDs, err := ansible.GetAnsibleHostsFromInventory(monitoringInventoryPath)
			if err != nil {
				return err
			}
			monitoringAnsibleHosts, err = ansible.GetHostMapfromAnsibleInventory(monitoringInventoryPath)
			if err != nil {
				return err
			}
			for _, id := range monitoringAnsibleHostIDs {
				_, ok := ansibleHosts[id]
				if !ok {
					ansibleHosts[id] = monitoringAnsibleHosts[id]
					ansibleHostIDs = append(ansibleHostIDs, id)
				}
			}
		}

		for _, ansibleHostID := range ansibleHostIDs {
			_, cloudHostID, err := models.HostAnsibleIDToCloudID(ansibleHostID)
			if err != nil {
				return err
			}
			nodeIDStr := "----------------------------------------"
			if clusterConf.MonitoringInstance != cloudHostID {
				nodeID, err := getNodeID(app.GetNodeInstanceDirPath(cloudHostID))
				if err != nil {
					return err
				}
				nodeIDStr = nodeID.String()
			}
			funcs := []string{}
			if clusterConf.IsAPIHost(ansibleHosts[ansibleHostID]) {
				funcs = append(funcs, "API")
			}
			if _, ok := monitoringAnsibleHosts[ansibleHostID]; ok {
				funcs = append(funcs, "MONITOR")
			}
			funcDesc := strings.Join(funcs, ",")
			if funcDesc != "" {
				funcDesc = " [" + funcDesc + "]"
			}
			ux.Logger.PrintToUser(fmt.Sprintf("  Node %s (%s) %s%s", cloudHostID, nodeIDStr, ansibleHosts[ansibleHostID].IP, funcDesc))
		}
	}
	return nil
}
