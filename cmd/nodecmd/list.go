// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
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
	clusterNames := maps.Keys(clustersConfig.Clusters)
	sort.Strings(clusterNames)
	for _, clusterName := range clusterNames {
		clusterConf := clustersConfig.Clusters[clusterName]
		ux.Logger.PrintToUser("Cluster %q (%s)", clusterName, clusterConf.Network.Name())
		if err := checkCluster(clusterName); err != nil {
			return err
		}
		for _, cloudID := range clusterConf.GetCloudIDs() {
			nodeConfig, err := app.LoadClusterNodeConfig(cloudID)
			if err != nil {
				return err
			}
			nodeIDStr := "----------------------------------------"
			funcs := []string{}
			if clusterConf.MonitoringInstance != cloudID {
				nodeID, err := getNodeID(app.GetNodeInstanceDirPath(cloudID))
				if err != nil {
					return err
				}
				nodeIDStr = nodeID.String()
				if clusterConf.IsAPIHost(cloudID) {
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
			ux.Logger.PrintToUser(fmt.Sprintf("  Node %s (%s) %s%s", cloudID, nodeIDStr, nodeConfig.ElasticIP, funcDesc))
		}
	}
	return nil
}
