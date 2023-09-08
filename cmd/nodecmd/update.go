// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Update nodes in a cluster with latest subnet configuration and virtual machine",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node update command updates all nodes in a cluster with latest Subnet configuration and virtual machine.
You can check the updated subnet bootstrap status by calling avalanche node status <clusterName> --subnet <subnetName>`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
		RunE:         updateSubnet,
	}

	return cmd
}

func updateSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	if err := checkCluster(clusterName); err != nil {
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
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	blockchainID := sc.Networks[models.Fuji.String()].BlockchainID
	if blockchainID == ids.Empty {
		return ErrNoBlockchainID
	}
	// the node is supposed to be synced to the subnet
	isSubnetSynced, err := getNodeSubnetSyncStatus(blockchainID.String(), clusterName, false, true)
	if err != nil {
		return err
	}
	if !isSubnetSynced {
		return errors.New("node is not synced to subnet")
	}
	if err := checkAvalancheGoVersionCompatible(clusterName, subnetName); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Installing Custom VM build environment on the EC2 instance ...")
	if err := ansible.RunAnsiblePlaybookSetupBuildEnv(app.GetAnsibleDir(), app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return err
	}
	if err := ansible.RunAnsiblePlaybookSetupCLIFromSource(app.GetAnsibleDir(), app.GetAnsibleInventoryPath(clusterName), constants.CloudCLIBranch); err != nil {
		return err
	}
	return doUpdateSubnet(clusterName, subnetName, models.Fuji)
}

// doUpdateSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// restart tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func doUpdateSubnet(clusterName, subnetName string, network models.Network) error {
	subnetPath := "/tmp/" + subnetName + constants.ExportSubnetSuffix
	if err := subnetcmd.CallExportSubnet(subnetName, subnetPath, network); err != nil {
		return err
	}
	if err := ansible.RunAnsiblePlaybookExportSubnet(app.GetAnsibleDir(), app.GetAnsibleInventoryPath(clusterName), subnetPath, "/tmp"); err != nil {
		return err
	}
	// runs avalanche join subnet command
	if err := ansible.RunAnsiblePlaybookUpdateSubnet(app.GetAnsibleDir(), subnetName, subnetPath, app.GetAnsibleInventoryPath(clusterName)); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Node successfully updated Subnet!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node subnet resyncing status with avalanche node status %s --subnet %s", clusterName, subnetName))
	return nil
}
