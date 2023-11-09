// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package nodecmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/ansible"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func checkClusterIsHealthy(clusterName string) ([]string, error) {
	ansibleNodeIDs, err := ansible.GetAnsibleHostsFromInventory(app.GetAnsibleInventoryDirPath(clusterName))
	if err != nil {
		return nil, err
	}
	notHealthyNodes := []string{}
	ux.Logger.PrintToUser(fmt.Sprintf("Checking if node(s) in cluster %s are healthy ...", clusterName))
	if err := app.CreateAnsibleStatusDir(); err != nil {
		return nil, err
	}
	if err := ansible.RunAnsiblePlaybookCheckHealthy(app.GetAnsibleDir(), app.GetHealthyJSONFile(), app.GetAnsibleInventoryDirPath(clusterName), "all"); err != nil {
		return nil, err
	}
	for _, ansibleNodeID := range ansibleNodeIDs {
		isHealthy, err := parseHealthyOutput(app.GetHealthyJSONFile() + "." + ansibleNodeID)
		if err != nil {
			return nil, err
		}
		if !isHealthy {
			notHealthyNodes = append(notHealthyNodes, ansibleNodeID)
		}
	}
	if err := app.RemoveAnsibleStatusDir(); err != nil {
		return nil, err
	}
	return notHealthyNodes, nil
}

func parseHealthyOutput(filePath string) (bool, error) {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
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
