// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"fmt"
	"sync"

	"github.com/ava-labs/avalanche-cli/cmd/blockchaincmd"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync [clusterName] [blockchainName]",
		Short: "(ALPHA Warning) Sync nodes in a cluster with a subnet",
		Long: `(ALPHA Warning) This command is currently in experimental mode.

The node sync command enables all nodes in a cluster to be bootstrapped to a Blockchain.
You can check the blockchain bootstrap status by calling avalanche node status <clusterName> --blockchain <blockchainName>`,
		Args: cobrautils.ExactArgs(2),
		RunE: syncSubnet,
	}

	cmd.Flags().StringSliceVar(&validators, "validators", []string{}, "sync subnet into given comma separated list of validators. defaults to all cluster nodes")
	cmd.Flags().BoolVar(&avoidChecks, "no-checks", false, "do not check for bootstrapped/healthy status or rpc compatibility of nodes against subnet")
	cmd.Flags().StringSliceVar(&subnetAliases, "subnet-aliases", nil, "subnet alias to be used for RPC calls. defaults to subnet blockchain ID")

	return cmd
}

func syncSubnet(_ *cobra.Command, args []string) error {
	clusterName := args[0]
	blockchainName := args[1]
	if err := checkCluster(clusterName); err != nil {
		return err
	}
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	if _, err := blockchaincmd.ValidateSubnetNameAndGetChains([]string{blockchainName}); err != nil {
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
		if err := checkHostsAreRPCCompatible(hosts, blockchainName); err != nil {
			return err
		}
	}
	if err := prepareSubnetPlugin(hosts, blockchainName); err != nil {
		return err
	}
	if err := trackSubnet(hosts, clusterName, clusterConfig.Network, blockchainName); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Node(s) successfully started syncing with Blockchain!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node blockchain syncing status with avalanche node status %s --blockchain %s", clusterName, blockchainName))
	return nil
}

// prepareSubnetPlugin creates subnet plugin to all nodes in the cluster
func prepareSubnetPlugin(hosts []*models.Host, blockchainName string) error {
	sc, err := app.LoadSidecar(blockchainName)
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
// start tracking the specified subnet (similar to avalanche subnet join <blockchainName> command)
func trackSubnet(
	hosts []*models.Host,
	clusterName string,
	network models.Network,
	blockchainName string,
) error {
	// load cluster config
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	// and get list of subnets
	allSubnets := utils.Unique(append(clusterConfig.Subnets, blockchainName))

	// load sidecar to get subnet blockchain ID
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	blockchainID := sc.Networks[network.Name()].BlockchainID

	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	subnetAliases := append([]string{blockchainName}, subnetAliases...)
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if err := ssh.RunSSHStopNode(host); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
			}

			if err := ssh.RunSSHRenderAvagoAliasConfigFile(
				host,
				blockchainID.String(),
				subnetAliases,
			); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
			}
			if err := ssh.RunSSHRenderAvalancheNodeConfig(
				app,
				host,
				network,
				allSubnets,
				clusterConfig.IsAPIHost(host.GetCloudID()),
			); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
			}
			if err := ssh.RunSSHSyncSubnetData(app, host, network, blockchainName); err != nil {
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
		return fmt.Errorf("failed to track subnet for node(s) %s", wgResults.GetErrorHostMap())
	}

	// update slice of subnets synced by the cluster
	clusterConfig.Subnets = allSubnets
	err = app.SetClusterConfig(network.ClusterName, clusterConfig)
	if err != nil {
		return err
	}

	// update slice of blockchain endpoints with the cluster ones
	networkInfo := sc.Networks[clusterConfig.Network.Name()]
	rpcEndpoints := set.Of(networkInfo.RPCEndpoints...)
	wsEndpoints := set.Of(networkInfo.WSEndpoints...)
	publicEndpoints, err := getPublicEndpoints(clusterName, hosts)
	if err != nil {
		return err
	}
	for _, publicEndpoint := range publicEndpoints {
		rpcEndpoints.Add(getRPCEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
		wsEndpoints.Add(getWSEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
	}
	networkInfo.RPCEndpoints = rpcEndpoints.List()
	networkInfo.WSEndpoints = wsEndpoints.List()
	sc.Networks[clusterConfig.Network.Name()] = networkInfo
	return app.UpdateSidecar(&sc)
}
