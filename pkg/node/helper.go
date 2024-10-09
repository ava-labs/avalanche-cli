// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package node

import (
	"encoding/json"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"sync"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/api/info"
)

const (
	HealthCheckPoolTime = 60 * time.Second
	HealthCheckTimeout  = 3 * time.Minute
)

func AuthorizedAccessFromSettings(app *application.Avalanche) bool {
	return app.Conf.GetConfigBoolValue(constants.ConfigAuthorizeCloudAccessKey)
}

func CheckCluster(app *application.Avalanche, clusterName string) error {
	_, err := GetClusterNodes(app, clusterName)
	return err
}

func GetClusterNodes(app *application.Avalanche, clusterName string) ([]string, error) {
	if exists, err := CheckClusterExists(app, clusterName); err != nil || !exists {
		return nil, fmt.Errorf("cluster %q not found", clusterName)
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return nil, err
	}
	clusterNodes := clustersConfig.Clusters[clusterName].Nodes
	if len(clusterNodes) == 0 {
		return nil, fmt.Errorf("no nodes found in cluster %s", clusterName)
	}
	return clusterNodes, nil
}

func CheckClusterExists(app *application.Avalanche, clusterName string) (bool, error) {
	clustersConfig := models.ClustersConfig{}
	if app.ClustersConfigExists() {
		var err error
		clustersConfig, err = app.LoadClustersConfig()
		if err != nil {
			return false, err
		}
	}
	_, ok := clustersConfig.Clusters[clusterName]
	return ok, nil
}

func CheckHostsAreRPCCompatible(app *application.Avalanche, hosts []*models.Host, subnetName string) error {
	incompatibleNodes, err := getRPCIncompatibleNodes(app, hosts, subnetName)
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
		ux.Logger.PrintToUser("Yoy can use \"avalanche node upgrade\" to upgrade Avalanche Go and/or Subnet-EVM to their latest versions")
		return fmt.Errorf("the Avalanche Go version of node(s) %s is incompatible with VM RPC version of %s", incompatibleNodes, subnetName)
	}
	return nil
}

func getRPCIncompatibleNodes(app *application.Avalanche, hosts []*models.Host, subnetName string) ([]string, error) {
	ux.Logger.PrintToUser("Checking compatibility of node(s) avalanche go RPC protocol version with Subnet EVM RPC of subnet %s ...", subnetName)
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return nil, err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if resp, err := ssh.RunSSHCheckAvalancheGoVersion(host); err != nil {
				nodeResults.AddResult(host.GetCloudID(), nil, err)
				return
			} else {
				if _, rpcVersion, err := ParseAvalancheGoOutput(resp); err != nil {
					nodeResults.AddResult(host.GetCloudID(), nil, err)
				} else {
					nodeResults.AddResult(host.GetCloudID(), rpcVersion, err)
				}
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return nil, fmt.Errorf("failed to get rpc protocol version for node(s) %s", wgResults.GetErrorHostMap())
	}
	incompatibleNodes := []string{}
	for nodeID, rpcVersionI := range wgResults.GetResultMap() {
		rpcVersion := rpcVersionI.(uint32)
		if rpcVersion != uint32(sc.RPCVersion) {
			incompatibleNodes = append(incompatibleNodes, nodeID)
		}
	}
	if len(incompatibleNodes) > 0 {
		ux.Logger.PrintToUser(fmt.Sprintf("Compatible Avalanche Go RPC version is %d", sc.RPCVersion))
	}
	return incompatibleNodes, nil
}

func ParseAvalancheGoOutput(byteValue []byte) (string, uint32, error) {
	reply := map[string]interface{}{}
	if err := json.Unmarshal(byteValue, &reply); err != nil {
		return "", 0, err
	}
	resultMap := reply["result"]
	resultJSON, err := json.Marshal(resultMap)
	if err != nil {
		return "", 0, err
	}

	nodeVersionReply := info.GetNodeVersionReply{}
	if err := json.Unmarshal(resultJSON, &nodeVersionReply); err != nil {
		return "", 0, err
	}
	return nodeVersionReply.VMVersions["platform"], uint32(nodeVersionReply.RPCProtocolVersion), nil
}

func DisconnectHosts(hosts []*models.Host) {
	for _, host := range hosts {
		_ = host.Disconnect()
	}
}

func getWSEndpoint(endpoint string, blockchainID string) string {
	return models.NewDevnetNetwork(endpoint, 0).BlockchainWSEndpoint(blockchainID)
}

func getPublicEndpoints(
	app *application.Avalanche,
	clusterName string,
	trackers []*models.Host,
) ([]string, error) {
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return nil, err
	}
	publicNodes := clusterConfig.APINodes
	if clusterConfig.Network.Kind == models.Devnet {
		publicNodes = clusterConfig.Nodes
	}
	publicTrackers := utils.Filter(trackers, func(tracker *models.Host) bool {
		return utils.Belongs(publicNodes, tracker.GetCloudID())
	})
	endpoints := utils.Map(publicTrackers, func(tracker *models.Host) string {
		return GetAvalancheGoEndpoint(tracker.IP)
	})
	return endpoints, nil
}

func getRPCEndpoint(endpoint string, blockchainID string) string {
	return models.NewDevnetNetwork(endpoint, 0).BlockchainEndpoint(blockchainID)
}

func GetAvalancheGoEndpoint(ip string) string {
	return fmt.Sprintf("http://%s:%d", ip, constants.AvalanchegoAPIPort)
}

func GetUnhealthyNodes(hosts []*models.Host) ([]string, error) {
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *models.Host) {
			defer wg.Done()
			if resp, err := ssh.RunSSHCheckHealthy(host); err != nil {
				nodeResults.AddResult(host.GetCloudID(), nil, err)
				return
			} else {
				if isHealthy, err := parseHealthyOutput(resp); err != nil {
					nodeResults.AddResult(host.GetCloudID(), nil, err)
				} else {
					nodeResults.AddResult(host.GetCloudID(), isHealthy, err)
				}
			}
		}(&wgResults, host)
	}
	wg.Wait()
	if wgResults.HasErrors() {
		return nil, fmt.Errorf("failed to get health status for node(s) %s", wgResults.GetErrorHostMap())
	}
	return utils.Filter(wgResults.GetNodeList(), func(nodeID string) bool {
		return !wgResults.GetResultMap()[nodeID].(bool)
	}), nil
}

func parseHealthyOutput(byteValue []byte) (bool, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(byteValue, &result); err != nil {
		return false, err
	}
	isHealthyInterface, ok := result["result"].(map[string]interface{})
	if ok {
		isHealthy, ok := isHealthyInterface["healthy"].(bool)
		if ok {
			return isHealthy, nil
		}
	}
	return false, fmt.Errorf("unable to parse node healthy status")
}

func WaitForHealthyCluster(
	app *application.Avalanche,
	clusterName string,
	timeout time.Duration,
	poolTime time.Duration,
) error {
	ux.Logger.PrintToUser("")
	ux.Logger.PrintToUser("Waiting for node(s) in cluster %s to be healthy...", clusterName)
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	cluster, ok := clustersConfig.Clusters[clusterName]
	if !ok {
		return fmt.Errorf("cluster %s does not exist", clusterName)
	}
	allHosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return err
	}
	hosts := cluster.GetValidatorHosts(allHosts) // exlude api nodes
	defer DisconnectHosts(hosts)
	startTime := time.Now()
	spinSession := ux.NewUserSpinner()
	spinner := spinSession.SpinToUser("Checking if node(s) are healthy...")
	for {
		unhealthyNodes, err := GetUnhealthyNodes(hosts)
		if err != nil {
			ux.SpinFailWithError(spinner, "", err)
			return err
		}
		if len(unhealthyNodes) == 0 {
			ux.SpinComplete(spinner)
			spinSession.Stop()
			ux.Logger.GreenCheckmarkToUser("Nodes healthy after %d seconds", uint32(time.Since(startTime).Seconds()))
			return nil
		}
		if time.Since(startTime) > timeout {
			ux.SpinFailWithError(spinner, "", fmt.Errorf("cluster not healthy after %d seconds", uint32(timeout.Seconds())))
			spinSession.Stop()
			ux.Logger.PrintToUser("")
			ux.Logger.RedXToUser("Unhealthy Nodes")
			for _, failedNode := range unhealthyNodes {
				ux.Logger.PrintToUser("  " + failedNode)
			}
			ux.Logger.PrintToUser("")
			return fmt.Errorf("cluster not healthy after %d seconds", uint32(timeout.Seconds()))
		}
		time.Sleep(poolTime)
	}
}
