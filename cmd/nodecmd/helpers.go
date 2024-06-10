// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-tooling-sdk-go/host"
	"github.com/ava-labs/avalanchego/api/info"
)

// NumNodes is a struct to hold number of nodes with and without stake
type NumNodes struct {
	numValidators int // with stake
	numAPI        int // without stake
}

func (n NumNodes) All() int {
	return n.numValidators + n.numAPI
}

func getUnhealthyNodes(hosts []*host.Host) ([]string, error) {
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *host.Host) {
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

func getNotBootstrappedNodes(hosts []*host.Host) ([]string, error) {
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *host.Host) {
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

func getRPCIncompatibleNodes(hosts []*host.Host, subnetName string) ([]string, error) {
	ux.Logger.PrintToUser("Checking compatibility of node(s) avalanche go RPC protocol version with Subnet EVM RPC of subnet %s ...", subnetName)
	sc, err := app.LoadSidecar(subnetName)
	if err != nil {
		return nil, err
	}
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host *host.Host) {
			defer wg.Done()
			if resp, err := ssh.RunSSHCheckAvalancheGoVersion(host); err != nil {
				nodeResults.AddResult(host.GetCloudID(), nil, err)
				return
			} else {
				if _, rpcVersion, err := parseAvalancheGoOutput(resp); err != nil {
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

func parseAvalancheGoOutput(byteValue []byte) (string, uint32, error) {
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

func disconnectHosts(hosts []*host.Host) {
	for _, host := range hosts {
		_ = host.Disconnect()
	}
}

func authorizedAccessFromSettings() bool {
	return app.Conf.GetConfigBoolValue(constants.ConfigAuthorizeCloudAccessKey)
}

func checkHostsAreRPCCompatible(hosts []*host.Host, subnetName string) error {
	incompatibleNodes, err := getRPCIncompatibleNodes(hosts, subnetName)
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

func checkHostsAreHealthy(hosts []*host.Host) error {
	ux.Logger.PrintToUser("Checking if node(s) are healthy...")
	unhealthyNodes, err := getUnhealthyNodes(hosts)
	if err != nil {
		return err
	}
	if len(unhealthyNodes) > 0 {
		return fmt.Errorf("node(s) %s are not healthy, please check the issue and try again later", unhealthyNodes)
	}
	return nil
}

func checkHostsAreBootstrapped(hosts []*host.Host) error {
	notBootstrappedNodes, err := getNotBootstrappedNodes(hosts)
	if err != nil {
		return err
	}
	if len(notBootstrappedNodes) > 0 {
		return fmt.Errorf("node(s) %s are not bootstrapped yet, please try again later", notBootstrappedNodes)
	}
	return nil
}
