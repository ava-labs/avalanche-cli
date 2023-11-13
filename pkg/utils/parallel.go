// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package utils

import (
	"sync"

	"github.com/ava-labs/avalanche-cli/pkg/models"
)

func RunInWaitGroupWithError(hosts []models.Host, functions []models.HostFuncWithParams) map[string]error {
	nodeResultChannel := make(chan models.NodeErrorResult, len(hosts))
	parallelWaitGroup := sync.WaitGroup{}
	for _, host := range hosts {
		parallelWaitGroup.Add(1)
		go func(nodeResultChannel chan models.NodeErrorResult, host models.Host) {
			defer parallelWaitGroup.Done()
			for _, f := range functions {
				if err := f.Run(host); err != nil {
					nodeResultChannel <- models.NodeErrorResult{NodeID: host.NodeID, Err: err}
					return
				}
			}
		}(nodeResultChannel, host)
	}
	parallelWaitGroup.Wait()
	close(nodeResultChannel)
	failedNodes := map[string]error{}
	for nodeErr := range nodeResultChannel {
		failedNodes[nodeErr.NodeID] = nodeErr.Err
	}
	return failedNodes
}
