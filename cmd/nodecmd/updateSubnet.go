// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
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
	if _, err := subnetcmd.ValidateSubnetNameAndGetChains([]string{subnetName}); err != nil {
		return err
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	defer disconnectHosts(hosts)
	notBootstrappedNodes, err := checkHostsAreBootstrapped(hosts)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	notHealthyNodes, err := checkHostsAreHealthy(hosts)
	if err != nil {
		return err
	}
	if len(notHealthyNodes) > 0 {
		return fmt.Errorf("node(s) %s are not healthy, please fix the issue and again", notHealthyNodes)
	}
	incompatibleNodes, err := checkAvalancheGoVersionCompatible(hosts, subnetName)
	if err != nil {
		return err
	}
	if len(incompatibleNodes) > 0 {
		sc, err := app.LoadSidecar(subnetName)
		if err != nil {
			return err
		}
		ux.Logger.PrintToUser("Either modify your Avalanche Go version or modify your VM version")
		ux.Logger.PrintToUser("To modify your Avalanche Go version: https://docs.avax.network/nodes/maintain/upgrade-your-avalanchego-node")
		switch sc.VM {
		case models.SubnetEvm:
			ux.Logger.PrintToUser("To modify your Subnet-EVM version: https://docs.avax.network/build/subnet/upgrade/upgrade-subnet-vm")
		case models.CustomVM:
			ux.Logger.PrintToUser("To modify your Custom VM binary: avalanche subnet upgrade vm %s --config", subnetName)
		}
		return fmt.Errorf("the Avalanche Go version of node(s) %s is incompatible with VM RPC version of %s", incompatibleNodes, subnetName)
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	network := clustersConfig.Clusters[clusterName].Network
	nonUpdatedNodes, err := doUpdateSubnet(hosts, subnetName, network)
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
	subnetName string,
	network models.Network,
) ([]string, error) {
	subnetPath := "/tmp/" + subnetName + constants.ExportSubnetSuffix
	if err := subnetcmd.CallExportSubnet(subnetName, subnetPath, network); err != nil {
		return nil, err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			subnetExportPath := filepath.Join("/tmp", filepath.Base(subnetPath))
			if err := ssh.RunSSHExportSubnet(host, subnetPath, subnetExportPath); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			if err := ssh.RunSSHUpdateSubnet(host, subnetName, subnetExportPath); err != nil {
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
