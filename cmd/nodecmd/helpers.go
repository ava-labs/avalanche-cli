// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ssh"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func checkClusterIsHealthy(clusterName string) ([]string, error) {
	hosts, err := ansible.GetInventoryFromAnsibleInventoryFile(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	ux.Logger.PrintToUser(fmt.Sprintf("Checking if node(s) in cluster %s are healthy ...", clusterName))
	wg := sync.WaitGroup{}
	wgResults := models.NodeResults{}
	for _, host := range hosts {
		wg.Add(1)
		go func(nodeResults *models.NodeResults, host models.Host) {
			defer wg.Done()
			if err := host.Connect(constants.SSHPOSTTimeout); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			}
			defer func() {
				if err := host.Disconnect(); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
				}
			}()
			if resp, err := ssh.RunSSHCheckHealthy(host); err != nil {
				nodeResults.AddResult(host.NodeID, nil, err)
				return
			} else {
				if isHealthy, err := parseHealthyOutput(resp); err != nil {
					nodeResults.AddResult(host.NodeID, nil, err)
				} else {
					nodeResults.AddResult(host.NodeID, isHealthy, err)
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
