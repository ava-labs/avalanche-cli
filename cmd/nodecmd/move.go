// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"github.com/spf13/cobra"
)

func newMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move [originClusterName] [destinationClusterName]",
		Short: "(ALPHA Warning) Moves nodes from a cluster to another cluster",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node move command moves node(s) from a cluster to another cluster.`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         moveNode,
	}

	return cmd
}

func moveNode(_ *cobra.Command, args []string) error {
	originClusterName := args[0]
	destinationClusterName := args[1]
	if err := checkCluster(originClusterName); err != nil {
		return err
	}
	//networkStr, err := app.Prompt.CaptureList(
	//	"Choose a network to deploy on",
	//	[]string{models.Local.String(), models.Fuji.String(), models.Mainnet.String()},
	//)
	return nil
}

func updateClusterConfig() {

}

func updateAnsibleInventory() {

}
