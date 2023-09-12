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
		for _, host := range ansibleHostIDs {
			isSubnetSynced, err := getNodeSubnetSyncStatus(blockchainID.String(), clusterName, host, true)
			if err != nil {
				return err
			}
			if !isSubnetSynced {
				notSyncedNodes = append(notSyncedNodes, host)
			}
		}
		printOutput(ansibleHostIDs, notSyncedNodes, clusterName, subnetName)
		return nil
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	fmt.Printf("obtained results %s \n", notBootstrappedNodes)
	printOutput(ansibleHostIDs, notBootstrappedNodes, clusterName, subnetName)
	return nil
}

func printOutput(hostAliases, notBootstrappedHosts []string, clusterName, subnetName string) {
	if len(notBootstrappedHosts) == 0 {
		if subnetName == "" {
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
				break
			}
		}
		isBootstrappedStr := "is not"
		if hostIsBootstrapped {
			isBootstrappedStr = "is"
		}
		if subnetName == "" {
			ux.Logger.PrintToUser(fmt.Sprintf("Node %s %s bootstrapped to Primary Network", host, isBootstrappedStr))
		} else {
			ux.Logger.PrintToUser(fmt.Sprintf("Node %s %s synced to Subnet %s", host, isBootstrappedStr, subnetName))
		}
	}
}
