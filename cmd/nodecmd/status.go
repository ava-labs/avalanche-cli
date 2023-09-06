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
	"github.com/spf13/cobra"
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
	hostAliases, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryFilePath(clusterName))
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
		for _, host := range hostAliases {
			isSubnetSynced, err := getNodeSubnetSyncStatus(blockchainID.String(), clusterName, host, true, false)
			if err != nil {
				return err
			}
			if !isSubnetSynced {
				notSyncedNodes = append(notSyncedNodes, host)
			}
		}
		printOutput(hostAliases, notSyncedNodes, clusterName, false)
		return nil
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	printOutput(hostAliases, notBootstrappedNodes, clusterName, true)
	return nil
}

func printOutput(hostAliases, notBootstrappedHosts []string, clusterName string, primaryNetwork bool) {
	if len(notBootstrappedHosts) == 0 {
		if primaryNetwork {
			ux.Logger.PrintToUser(fmt.Sprintf("All nodes in cluster %s are bootstrapped to Primary Network!", clusterName))
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("All nodes in cluster %s are synced to Subnet %s", clusterName, subnetName))
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
			}
			break
		}
		if hostIsBootstrapped {
			if primaryNetwork {
				ux.Logger.PrintToUser(fmt.Sprintf("Node %s is bootstrapped to Primary Network", host))
			} else {
				ux.Logger.PrintToUser(fmt.Sprintf("Node %s is synced to Subnet %s", host, subnetName))
			}
		} else {
			if primaryNetwork {
				ux.Logger.PrintToUser(fmt.Sprintf("Node %s is not bootstrapped to Primary Network", host))
			} else {
				ux.Logger.PrintToUser(fmt.Sprintf("Node %s is not synced to Subnet %s", host, subnetName))
			}
		}
	}
}
