// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package models

import "sync"

type NodeResult struct {
	NodeID string
	Value  interface{}
	Err    error
}
type NodeResults struct {
	Results []NodeResult
	Lock    sync.Mutex
}

func (nr *NodeResults) AddResult(nodeID string, value interface{}, err error) {
	nr.Lock.Lock()
	defer nr.Lock.Unlock()
	nr.Results = append(nr.Results, NodeResult{
		NodeID: nodeID,
		Value:  value,
		Err:    err,
	})
}

func (nr *NodeResults) GetResults() []NodeResult {
	nr.Lock.Lock()
	defer nr.Lock.Unlock()
	return nr.Results
}

func (nr *NodeResults) GetResultMap() map[string]interface{} {
	nr.Lock.Lock()
	defer nr.Lock.Unlock()
	result := map[string]interface{}{}
	for _, node := range nr.Results {
		result[node.NodeID] = node.Value
	}
	return result
}

func (nr *NodeResults) Len() int {
	nr.Lock.Lock()
	defer nr.Lock.Unlock()
	return len(nr.Results)
}

func (nr *NodeResults) GetNodeList() []string {
	nr.Lock.Lock()
	defer nr.Lock.Unlock()
	nodes := []string{}
	for _, node := range nr.Results {
		nodes = append(nodes, node.NodeID)
	}
	return nodes
}

func (nr *NodeResults) GetErrorHostMap() map[string]error {
	nr.Lock.Lock()
	defer nr.Lock.Unlock()
	hostErrors := make(map[string]error)
	for _, node := range nr.Results {
		if node.Err != nil {
			hostErrors[node.NodeID] = node.Err
		}
	}
	return hostErrors
}

func (nr *NodeResults) HasNodeIDWithError(nodeID string) bool {
	nr.Lock.Lock()
	defer nr.Lock.Unlock()
	for _, node := range nr.Results {
		if node.NodeID == nodeID && node.Err != nil {
			return true
		}
	}
	return false
}

func (nr *NodeResults) HasErrors() bool {
	return len(nr.GetErrorHostMap()) > 0
}

func (nr *NodeResults) GetErrorHosts() []string {
	var nodes []string
	for _, node := range nr.Results {
		if node.Err != nil {
			nodes = append(nodes, node.NodeID)
		}
	}
	return nodes
}
