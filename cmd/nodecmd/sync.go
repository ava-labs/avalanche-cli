// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"sync"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [clusterName] [subnetName]",
		Short: "(ALPHA Warning) Sync nodes in a cluster with a subnet",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Subnet. 
You can check the subnet bootstrap status by calling avalanche node status <clusterName> --subnet <subnetName>`,
		Args: cobrautils.ExactArgs(2),
		RunE: syncSubnet,
	}

	cmd.Flags().StringSliceVar(&validators, "validators", []string{}, "sync subnet into given comma separated list of validators. defaults to all cluster nodes")
	cmd.Flags().BoolVar(&avoidChecks, "no-checks", false, "do not check for bootstrapped/healthy status or rpc compatibility of nodes against subnet")
	cmd.Flags().StringVar(&subnetAlias, "subnet-alias", "", "subnet alias to be used for RPC calls. defaults to subnet blockchain ID")

	return cmd
}

func syncSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	subnetName := args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	if len(validators) != 0 {
		hosts, err = filterHosts(hosts, validators)
		if err != nil {
			return err
		}
	}
	defer disconnectHosts(hosts)
	if !avoidChecks {
		if err := checkHostsAreBootstrapped(hosts); err != nil {
			return err
		}
		if err := checkHostsAreHealthy(hosts); err != nil {
			return err
		}
		if err := checkHostsAreRPCCompatible(hosts, subnetName); err != nil {
			return err
		}
	}
	if err := prepareSubnetPlugin(hosts, subnetName); err != nil {
		return err
	}
	untrackedNodes, err := trackSubnet(hosts, clusterName, clusterConfig.Network, subnetName, subnetAlias)
	if err != nil {
		return err
	}
	if len(untrackedNodes) > 0 {
		return fmt.Errorf("node(s) %s failed to sync with subnet %s", untrackedNodes, subnetName)
	}
	ux.Logger.PrintToUser("Node(s) successfully started syncing with Subnet!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node subnet syncing status with avalanche node status %s --subnet %s", clusterName, subnetName))
	return nil
}

// prepareSubnetPlugin creates subnet plugin to all nodes in the cluster
func prepareSubnetPlugin(hosts []*models.Host, subnetName string) error {
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if err := ssh.RunSSHCreatePlugin(host, sc); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return fmt.Errorf("failed to upload plugin to node(s) %s", wgResults.GetErrorHostMap())
	}
	return nil
}

// trackSubnet exports deployed subnet in user's local machine to cloud server and calls node to
// start tracking the specified subnet (similar to avalanche subnet join <subnetName> command)
func trackSubnet(
	hosts []*models.Host,
	clusterName string,
	network models.Network,
	subnetName string,
	subnetAlias string,
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
			if err := ssh.RunSSHRenderAvalancheNodeConfig(app, host, network, allSubnets); err != nil {
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
		return nil, fmt.Errorf("failed to track subnet for node(s) %s", wgResults.GetErrorHostMap())
	}
	return wgResults.GetErrorHosts(), nil
}
