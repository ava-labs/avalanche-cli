// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"sort"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/node"

	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
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
		Args: cobrautils.ExactArgs(0),
		RunE: list,
	}

	return cmd
}

func list(_ *cobra.Command, _ []string) error {
	clustersConfig, err := app.GetClustersConfig()
	if err != nil {
		return err
	}
	if len(clustersConfig.Clusters) == 0 {
		ux.Logger.PrintToUser("There are no clusters defined.")
	}
	clusterNames := maps.Keys(clustersConfig.Clusters)
	sort.Strings(clusterNames)
	for _, clusterName := range clusterNames {
		clusterConf := clustersConfig.Clusters[clusterName]
		if err := node.CheckCluster(app, clusterName); err != nil {
			return err
		}
		cloudIDs := clusterConf.GetCloudIDs()
		nodeIDs := []string{}
		for _, cloudID := range cloudIDs {
			nodeIDStr := "----------------------------------------"
			if clusterConf.IsAvalancheGoHost(cloudID) {
				if nodeID, err := getNodeID(app.GetNodeInstanceDirPath(cloudID)); err != nil {
					ux.Logger.RedXToUser("could not obtain node ID for nodes %s: %s", cloudID, err)
				} else {
					nodeIDStr = nodeID.String()
				}
			}
			nodeIDs = append(nodeIDs, nodeIDStr)
		}
		switch {
		case clusterConf.External:
			ux.Logger.PrintToUser("cluster %q (%s) EXTERNAL", clusterName, clusterConf.Network.Type.String())
		case clusterConf.Local:
			ux.Logger.PrintToUser("cluster %q (%s) LOCAL", clusterName, clusterConf.Network.Type.String())
		default:
			ux.Logger.PrintToUser("Cluster %q (%s)", clusterName, clusterConf.Network.Type.String())
		}
		for i, cloudID := range clusterConf.GetCloudIDs() {
			nodeConfig, err := app.LoadClusterNodeConfig(cloudID)
			if err != nil {
				return err
			}
			roles := clusterConf.GetHostRoles(nodeConfig)
			rolesStr := strings.Join(roles, ",")
			if rolesStr != "" {
				rolesStr = " [" + rolesStr + "]"
			}
			ux.Logger.PrintToUser("  Node %s (%s) %s%s", cloudID, nodeIDs[i], nodeConfig.ElasticIP, rolesStr)
		}
	}
	return nil
}
