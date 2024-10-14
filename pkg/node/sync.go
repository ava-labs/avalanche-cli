// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-network-runner/client"
	anrutils "github.com/ava-labs/avalanche-network-runner/utils"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/set"
)

func SyncSubnet(app *application.Avalanche, clusterName, blockchainName string, avoidChecks bool, subnetAliases []string) error {
	if err := CheckCluster(app, clusterName); err != nil {
		return err
	}
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return err
	}
	if _, err := subnet.ValidateSubnetNameAndGetChains(app, []string{blockchainName}); err != nil {
		return err
	}
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	defer DisconnectHosts(hosts)
	if !avoidChecks {
		if err := CheckHostsAreBootstrapped(hosts); err != nil {
			return err
		}
		if err := CheckHostsAreHealthy(hosts); err != nil {
			return err
		}
		if err := CheckHostsAreRPCCompatible(app, hosts, blockchainName); err != nil {
			return err
		}
	}
	if err := prepareSubnetPlugin(app, hosts, blockchainName); err != nil {
		return err
	}
	if err := trackSubnet(app, hosts, clusterName, clusterConfig.Network, blockchainName, subnetAliases); err != nil {
		return err
	}
	ux.Logger.PrintToUser("Node(s) successfully started syncing with blockchain!")
	ux.Logger.PrintToUser(fmt.Sprintf("Check node blockchain syncing status with avalanche node status %s --blockchain %s", clusterName, blockchainName))
	return nil
}

// prepareSubnetPlugin creates subnet plugin to all nodes in the cluster
func prepareSubnetPlugin(app *application.Avalanche, hosts []*models.Host, blockchainName string) error {
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

func trackSubnet(
	app *application.Avalanche,
	hosts []*models.Host,
	clusterName string,
	network models.Network,
	blockchainName string,
	subnetAliases []string,
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
	subnetAliases = append([]string{blockchainName}, subnetAliases...)
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
	publicEndpoints, err := getPublicEndpoints(app, clusterName, hosts)
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

func CheckHostsAreBootstrapped(hosts []*models.Host) error {
	notBootstrappedNodes, err := GetNotBootstrappedNodes(hosts)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	return nil
}

func CheckHostsAreHealthy(hosts []*models.Host) error {
	ux.Logger.PrintToUser("Checking if node(s) are healthy...")
	unhealthyNodes, err := GetUnhealthyNodes(hosts)
	if err != nil {
		return err
	}
	if len(unhealthyNodes) > 0 {
		return fmt.Errorf("node(s) %s are not healthy, please check the issue and try again later", unhealthyNodes)
	}
	return nil
}

func GetNotBootstrappedNodes(hosts []*models.Host) ([]string, error) {
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if resp, err := ssh.RunSSHCheckBootstrapped(host); err != nil {
				nodeResults.AddResult(host.GetCloudID(), nil, err)
				return
			} else {
				if isBootstrapped, err := parseBootstrappedOutput(resp); err != nil {
					nodeResults.AddResult(host.GetCloudID(), nil, err)
				} else {
					nodeResults.AddResult(host.GetCloudID(), isBootstrapped, err)
				}
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return nil, fmt.Errorf("failed to get avalanchego bootstrap status for node(s) %s", wgResults.GetErrorHostMap())
	}
	return utils.Filter(wgResults.GetNodeList(), func(nodeID string) bool {
		return !wgResults.GetResultMap()[nodeID].(bool)
	}), nil
}

func parseBootstrappedOutput(byteValue []byte) (bool, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(byteValue, &result); err != nil {
		return false, err
	}
	isBootstrappedInterface, ok := result["result"].(map[string]interface{})
	if ok {
		isBootstrapped, ok := isBootstrappedInterface["isBootstrapped"].(bool)
		if ok {
			return isBootstrapped, nil
		}
	}
	return false, errors.New("unable to parse node bootstrap status")
}

func TrackSubnetWithLocalMachine(app *application.Avalanche, clusterName, blockchainName string) error {
	if ok, err := CheckClusterIsLocal(app, clusterName); err != nil || !ok {
		return fmt.Errorf("local node %q is not found", clusterName)
	}
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	network := clusterConfig.Network
	if sc.Networks[network.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, network.Name())
	}
	subnetID := sc.Networks[network.Name()].SubnetID
	chainVMID, err := anrutils.VMID(blockchainName)
	if err != nil {
		return fmt.Errorf("failed to create VM ID from %s: %w", blockchainName, err)
	}
	var vmBin string
	switch sc.VM {
	case models.SubnetEvm:
		_, vmBin, err = binutils.SetupSubnetEVM(app, sc.VMVersion)
		if err != nil {
			return fmt.Errorf("failed to install subnet-evm: %w", err)
		}
	case models.CustomVM:
		vmBin = binutils.SetupCustomBin(app, blockchainName)
	default:
		return fmt.Errorf("unknown vm: %s", sc.VM)
	}
	binaryDownloader := binutils.NewPluginBinaryDownloader(app)
	if err := binaryDownloader.InstallVM(chainVMID.String(), vmBin); err != nil {
		return err
	}
	cli, err := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}
	ctx, cancel := utils.GetANRContext()
	defer cancel()
	status, err := cli.Status(ctx)
	if err != nil {
		return err
	}
	publicEndpoints := []string{}
	for _, nodeInfo := range status.ClusterInfo.NodeInfos {
		if _, err := cli.RestartNode(ctx, nodeInfo.Name, client.WithWhitelistedSubnets(subnetID.String())); err != nil {
			return err
		}
		publicEndpoints = append(publicEndpoints, nodeInfo.Uri)
	}
	_, err = cli.WaitForHealthy(ctx)
	if err != nil {
		return err
	}
	networkInfo := sc.Networks[network.Name()]
	rpcEndpoints := set.Of(networkInfo.RPCEndpoints...)
	wsEndpoints := set.Of(networkInfo.WSEndpoints...)
	for _, publicEndpoint := range publicEndpoints {
		rpcEndpoints.Add(getRPCEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
		wsEndpoints.Add(getWSEndpoint(publicEndpoint, networkInfo.BlockchainID.String()))
	}
	networkInfo.RPCEndpoints = rpcEndpoints.List()
	networkInfo.WSEndpoints = wsEndpoints.List()
	sc.Networks[clusterConfig.Network.Name()] = networkInfo
	if err := app.UpdateSidecar(&sc); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("%s successfully tracking %s", clusterName, blockchainName)
	return nil
}

func CheckClusterIsLocal(app *application.Avalanche, clusterName string) (bool, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return false, err
		}
	}
	clusterConf, ok := clustersConfig.Clusters[clusterName]
	return ok && clusterConf.Local, nil
}
