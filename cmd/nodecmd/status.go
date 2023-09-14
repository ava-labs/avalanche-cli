// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [clusterName]",
		Short: "(ALPHA Warning) Get node bootstrap status",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node status command gets the bootstrap status of all nodes in a cluster with the Primary Network. 
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
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	if err := setupAnsible(); err != nil {
		return err
	}
	ansibleHostIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	if subnetName != "" {
		if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
			return err
		}
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}
		blockchainID := sc.Networks[models.Fuji.String()].BlockchainID
		if blockchainID == ids.Empty {
			return ErrNoBlockchainID
		}
		notSyncedNodes := []string{}
		subnetSyncedNodes := []string{}
		subnetValidatingNodes := []string{}
		for _, host := range ansibleHostIDs {
			subnetSyncStatus, err := getNodeSubnetSyncStatus(blockchainID.String(), clusterName, host)
			if err != nil {
				return err
			}
			switch subnetSyncStatus {
			case status.Syncing.String():
				subnetSyncedNodes = append(subnetSyncedNodes, host)
			case status.Validating.String():
				subnetValidatingNodes = append(subnetValidatingNodes, host)
			default:
				notSyncedNodes = append(notSyncedNodes, host)
			}
		}
		printOutput(ansibleHostIDs, notSyncedNodes, subnetSyncedNodes, subnetValidatingNodes, clusterName, subnetName)
		return nil
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	printOutput(ansibleHostIDs, notBootstrappedNodes, nil, nil, clusterName, subnetName)
	return nil
}

func printOutput(hostAliases, notBootstrappedHosts, subnetSyncedHosts, subnetValidatingHosts []string, clusterName, subnetName string) {
	if len(notBootstrappedHosts) == 0 {
		if subnetName == "" {
			ux.Logger.PrintToUser(fmt.Sprintf("All nodes in cluster %s are bootstrapped to Primary Network!", clusterName))
		} else {
			status := "synced"
			if len(subnetSyncedHosts) == 0 && len(subnetValidatingHosts) > 0 {
				status = "validators"
			}
			ux.Logger.PrintToUser(fmt.Sprintf("All nodes in cluster %s are %s to Subnet %s", clusterName, status, subnetName))
		}
		return
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Node(s) Status For Cluster %s", clusterName))
	ux.Logger.PrintToUser("======================================")
	for _, host := range hostAliases {
		hostIsBootstrapped := true
		for _, notBootstrappedHost := range notBootstrappedHosts {
			if notBootstrappedHost == host {
				hostIsBootstrapped = false
				break
			}
		}
		isBootstrappedStr := "is not"
		if hostIsBootstrapped {
			isBootstrappedStr = "is"
		}
		hostStatus := "synced"
		if slices.Contains(subnetValidatingHosts, host) {
			hostStatus = "validator"
		}
		if subnetName == "" {
			ux.Logger.PrintToUser(fmt.Sprintf("Node %s %s bootstrapped to Primary Network", host, isBootstrappedStr))
		} else {
			if !hostIsBootstrapped {
				ux.Logger.PrintToUser(fmt.Sprintf("Node %s is not synced to Subnet %s", host, subnetName))
			} else {
				ux.Logger.PrintToUser(fmt.Sprintf("Node %s is %s to Subnet %s", host, hostStatus, subnetName))
			}
		}
	}
}
