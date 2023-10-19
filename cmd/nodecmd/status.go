// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"sync"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var subnetName string

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
	if err := setupAnsible(clusterName); err != nil {
		return err
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Collecting data for node(s) in cluster %s ...", clusterName))
	avalanchegoVersionForNode := map[string]string{}
	ansibleHostIDs, err := ansible.GetHostListFromAnsibleInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	nodeResultChannel := make(chan models.NodeStringResult, len(hosts)) 
	parallelWaitGroup := sync.WaitGroup{}
	for _, host := range hosts {
		parallelWaitGroup.Add(1)
		go func(avalancheGoVersionCh chan models.NodeStringResult) {
			defer parallelWaitGroup.Done()
			resp, err := ssh.RunSSHCheckAvalancheGoVersion(host)
			if err != nil {
				avalancheGoVersionCh <- models.NodeStringResult{NodeID: host.NodeID, Value: constants.AvalancheGoVersionUnknown,Err:err}
			}
			avalancheGoVersion, err := parseAvalancheGoOutput(resp)
			if err != nil {
				avalancheGoVersionCh <- models.NodeStringResult{NodeID: host.NodeID, Value: constants.AvalancheGoVersionUnknown,Err:err}
			}
			avalancheGoVersionCh <- models.NodeStringResult{NodeID: host.NodeID, Value: avalancheGoVersion, Err: nil}
		}(nodeResultChannel)
	}
	parallelWaitGroup.Wait()
	close(nodeResultChannel)
	for avalancheGoVersionResult := range nodeResultChannel {
		if avalancheGoVersionResult.Err != nil {
			return avalancheGoVersionResult.Err
		}
		avalanchegoVersionForNode[avalancheGoVersionResult.NodeID] = avalancheGoVersionResult.Value
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
		nodeResultChannel := make(chan models.NodeStringResult, len(hosts)) 
		parallelWaitGroup := sync.WaitGroup{}
		for _, host := range hosts {
			parallelWaitGroup.Add(1)
			go func(SubnetSyncStatusCh chan  models.NodeStringResult) {
				defer parallelWaitGroup.Done()
				subnetSyncStatus, err := getNodeSubnetSyncStatus(blockchainID.String(), clusterName, host)
				if err != nil {
					nodeResultChannel <-  models.NodeStringResult{NodeID: host.NodeID, Value: "", Err: err}
				}
				nodeResultChannel <-  models.NodeStringResult{NodeID: host.NodeID, Value: subnetSyncStatus, Err: nil}
			}(nodeResultChannel)
		}
		parallelWaitGroup.Wait()
		close(nodeResultChannel)
		for SubnetSyncStatusResult := range nodeResultChannel {
			if SubnetSyncStatusResult.Err != nil {
				return SubnetSyncStatusResult.Err
			}
			switch SubnetSyncStatusResult.Value {
			case status.Syncing.String():
				subnetSyncedNodes = append(subnetSyncedNodes, SubnetSyncStatusResult.NodeID)
			case status.Validating.String():
				subnetValidatingNodes = append(subnetValidatingNodes, SubnetSyncStatusResult.NodeID)
			default:
				notSyncedNodes = append(notSyncedNodes, SubnetSyncStatusResult.NodeID)
			}
		}
		printOutput(avalanchegoVersionForNode, ansibleHostIDs, notSyncedNodes, subnetSyncedNodes, subnetValidatingNodes, clusterName, subnetName)
		return nil
	}
	notBootstrappedNodes, err := checkClusterIsBootstrapped(clusterName)
	if err != nil {
		return err
	}
	printOutput(avalanchegoVersionForNode, ansibleHostIDs, notBootstrappedNodes, nil, nil, clusterName, subnetName)
	return nil
}

func printOutput(hostAvalanchegoVersions map[string]string, hostAliases, notBootstrappedHosts, subnetSyncedHosts, subnetValidatingHosts []string, clusterName, subnetName string) {
	if len(notBootstrappedHosts) == 0 {
		if subnetName == "" {
			ux.Logger.PrintToUser("All nodes in cluster %s are bootstrapped to Primary Network!", clusterName)
		} else {
			// all nodes are either synced to or validating subnet
			status := "synced to"
			if len(subnetSyncedHosts) == 0 {
				status = "validators of"
			}
			ux.Logger.PrintToUser("All nodes in cluster %s are %s Subnet %s", clusterName, status, subnetName)
		}
		return
	}
	ux.Logger.PrintToUser("Node(s) Status For Cluster %s", clusterName)
	ux.Logger.PrintToUser("======================================")
	for _, host := range hostAliases {
		hostWithVersion := fmt.Sprintf("%s (%s)", host, hostAvalanchegoVersions[host])
		hostIsBootstrapped := true
		if slices.Contains(notBootstrappedHosts, host) {
			hostIsBootstrapped = false
		}
		hostStatus := "synced to"
		if slices.Contains(subnetValidatingHosts, host) {
			hostStatus = "validator of"
		}
		if subnetName == "" {
			isBootstrappedStr := "is not"
			if hostIsBootstrapped {
				isBootstrappedStr = "is"
			}
			ux.Logger.PrintToUser("Node %s %s bootstrapped to Primary Network", hostWithVersion, isBootstrappedStr)
		} else {
			if !hostIsBootstrapped {
				ux.Logger.PrintToUser("Node %s is not synced to Subnet %s", hostWithVersion, subnetName)
			} else {
				ux.Logger.PrintToUser("Node %s is %s Subnet %s", hostWithVersion, hostStatus, subnetName)
			}
		}
	}
}
