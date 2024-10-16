// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/node"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newUpdateSubnetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subnet [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Update nodes in a cluster with latest subnet configuration and VM for custom VM",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node update subnet command updates all nodes in a cluster with latest Subnet configuration and VM for custom VM.
You can check the updated subnet bootstrap status by calling avalanche node status <clusterName> --subnet <subnetName>`,
		Args: cobrautils.ExactArgs(2),
		RunE: updateSubnet,
	}

	return cmd
}

func updateSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	if err := node.CheckCluster(app, clusterName); err != nil {
		return err
	}
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	if clusterConfig.Local {
		return notImplementedForLocal("update")
	}
	if _, err := blockchaincmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	defer node.DisconnectHosts(hosts)
	if err := node.CheckHostsAreBootstrapped(hosts); err != nil {
		return err
	}
	if err := node.CheckHostsAreHealthy(hosts); err != nil {
		return err
	}
	if err := node.CheckHostsAreRPCCompatible(app, hosts, subnetName); err != nil {
		return err
	}
	nonUpdatedNodes, err := doUpdateSubnet(hosts, clusterName, clusterConfig.Network, subnetName)
	if err != nil {
		return err
	}
	if len(nonUpdatedNodes) > 0 {
		return fmt.Errorf("node(s) %s failed to be updated for subnet %s", nonUpdatedNodes, subnetName)
	}
	ux.Logger.PrintToUser("Node(s) successfully updated for Subnet!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node subnet status with avalanche node status %s --subnet %s", clusterName, subnetName))
	return nil
}

// doUpdateSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// restart tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func doUpdateSubnet(
	hosts []*models.Host,
	clusterName string,
	network models.Network,
	subnetName string,
) ([]string, error) {
	// load cluster config
	clusterConf, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return nil, err
	}
	// and get list of subnets
	allSubnets := utils.Unique(append(clusterConf.Subnets, subnetName))

	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if err := ssh.RunSSHStopNode(host); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
			}
			if err := ssh.RunSSHRenderAvalancheNodeConfig(
				app,
				host,
				network,
				allSubnets,
				clusterConf.IsAPIHost(host.GetCloudID()),
			); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
			}
			if err := ssh.RunSSHSyncSubnetData(app, host, network, subnetName); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
			}
			if err := ssh.RunSSHStartNode(host); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return nil, fmt.Errorf("failed to update subnet for node(s) %s", wgResults.GetErrorHostMap())
	}
	return wgResults.GetErrorHosts(), nil
}
