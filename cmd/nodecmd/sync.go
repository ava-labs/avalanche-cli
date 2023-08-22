// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	subnetcmd "github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [clusterName]",
		Short: "(ALPHA Warning) Sync nodes in a cluster with a subnet",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Subnet. 
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
	return trackSubnet(clusterName, subnetName, models.Fuji)
}

// trackSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// start tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func trackSubnet(clusterName, subnetToTrack string, network models.Network) error {
	subnetPath := "/tmp/" + subnetName + constants.ExportSubnetSuffix
	if err := subnetcmd.CallExportSubnet(subnetToTrack, subnetPath, network); err != nil {
		return err
	}
	if err := ansible.RunAnsiblePlaybookExportSubnet(app.GetAnsibleDir(), app.GetAnsibleInventoryPath(clusterName), subnetPath, "/tmp"); err != nil {
		return err
	}
	// runs avalanche join subnet command
	if err := ansible.RunAnsiblePlaybookTrackSubnet(app.GetAnsibleDir(), subnetToTrack, subnetPath, app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Node successfully started syncing with Subnet!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node subnet syncing status with avalanche node status %s --subnet %s", clusterName, subnetToTrack))
	return nil
}
