// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/spf13/cobra"
)

var clusterFileName string

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "(ALPHA Warning) Export cluster configuration to a file",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node export command export clusters configuration including their nodes to a text file.
If no file is specified, the configuration is printed to the stdout.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         export,
	}
	cmd.Flags().StringVar(&clusterFileName, "file", "", "specify the file to export the cluster configuration to")
	return cmd
}

type exportCluster struct {
	clusterConfig models.ClusterConfig
	nodes         []models.NodeConfig
}

func export(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if err := checkCluster(clusterName); err != nil {
		ux.Logger.RedXToUser("cluster not found: %w", err)
		return err
	}
	clusterConf, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}

	nodes, err := utils.MapWithError(clusterConf.Nodes, func(nodeName string) (models.NodeConfig, error) { return app.LoadClusterNodeConfig(nodeName) })
	if err != nil {
		ux.Logger.RedXToUser("could not load node configuration: %s", err)
		return err
	}
	exportCluster := exportCluster{
		clusterConfig: clusterConf,
		nodes:         nodes,
	}
	return nil
}
