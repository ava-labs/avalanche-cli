// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	subnetcmd "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [subnetName]",
		Short: "Sync with a subnet",
		Long: `The node sync command enables a node to to also be bootstrapped to a Subnet. 
You can check the subnet bootstrap status by calling avalanche node status --subnet`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE:         syncSubnet,
	}
	cmd.Flags().StringVar(&subnetName, "subnet", "", "specify the subnet the node is syncing with")

	return cmd
}

func syncSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	if subnetName == "" {
		ux.Logger.PrintToUser("Please provide the name of the subnet that the node will be validating with --subnet flag")
		return errors.New("no subnet provided")
	}
	_, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName})
	if err != nil {
		return err
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

func trackSubnet(clusterName string, network models.Network) error {
	err := subnetcmd.CallExportSubnet(subnetName, network)
	if err != nil {
		return err
	}
	inventoryPath := "inventories/" + clusterName
	err = ansible.RunAnsiblePlaybookExportSubnet(subnetName, inventoryPath)
	if err != nil {
		return err
	}
	err = ansible.RunAnsiblePlaybookTrackSubnet(subnetName, inventoryPath)
	if err != nil {
		return err
	}

	return nil
}
