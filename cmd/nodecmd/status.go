// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [subnetName]",
		Short: "(ALPHA Warning) Get node bootstrap status",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node status command gets the bootstrap status of a node with the Primary Network. 
To get the bootstrap status of a node with a Subnet, use --subnet flag`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         statusSubnet,
	}
	cmd.Flags().StringVar(&subnetName, "subnet", "", "specify the subnet the node is syncing with")

	return cmd
}

func statusSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	err := setupAnsible()
	if err != nil {
		return err
	}
	if subnetName != "" {
		_, err = subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName})
		if err != nil {
			return err
		}
	}

	isBootstrapped, err := checkNodeIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	if !isBootstrapped {
		return errors.New("node is not bootstrapped yet, please try again later")
	}
	err = trackSubnet(clusterName, models.Fuji)
	if err != nil {
		return err
	}
	return nil
}
