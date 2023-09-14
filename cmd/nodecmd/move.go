// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
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
	if err := setupAnsible(); err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	isBootstrapped, err := checkNodeIsBootstrapped(clusterName, false)
	if err != nil {
		return err
	}
	if !isBootstrapped {
		return errors.New("node is not bootstrapped yet, please try again later")
	}
	if err := checkAvalancheGoVersionCompatible(clusterName, subnetName); err != nil {
		return err
	}
	return trackSubnet(clusterName, subnetName, models.Fuji)
}

func updateClusterConfig() {

}

func updateAnsibleInventory() {

}