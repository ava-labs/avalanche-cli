// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/cmd/mocks"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TODO add some tests here
func TestDeployToLocal(t *testing.T) {
	assert := assert.New(t)

	log = logging.NoLog{}
	procChecker := &mocks.ProcessChecker{}
	procChecker.On("IsServerProcessRunning").Return(true, nil)

	binChecker := &mocks.BinaryChecker{}
	tmpDir := t.TempDir()
	err := os.Mkdir(filepath.Join(tmpDir, "plugins"), perms.ReadWriteExecute)
	assert.NoError(err)

	f, err := os.Create(filepath.Join(tmpDir, "avalanchego"))
	defer f.Close()
	assert.NoError(err)

	binChecker.On("Exists", mock.AnythingOfType("string")).Return(true, tmpDir, nil)

	binDownloader := &mocks.BinaryDownloader{}
	binDownloader.On("Download", mock.AnythingOfType("ids.ID"), mock.AnythingOfType("string")).Return(nil)

	testDeployer := &subnetDeployer{
		procChecker:         procChecker,
		binChecker:          binChecker,
		getClientFunc:       getTestClientFunc,
		binaryDownloader:    binDownloader,
		healthCheckInterval: 500 * time.Millisecond,
	}

	testGenesis, err := os.CreateTemp(tmpDir, "test-genesis.json")
	assert.NoError(err)
	err = testDeployer.deployToLocalNetwork("test", testGenesis.Name())
	assert.NoError(err)
}

func getTestClientFunc() (client.Client, error) {
	c := &mocks.Client{}
	fakeStartResponse := &rpcpb.StartResponse{}
	c.On("Start", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fakeStartResponse, nil)
	fakeHealthResponse := &rpcpb.HealthResponse{
		ClusterInfo: &rpcpb.ClusterInfo{
			NodeInfos: map[string]*rpcpb.NodeInfo{
				"testNode1": {
					Name: "testNode1",
					Uri:  "http://fake.localhost:12345",
				},
				"testNode2": {
					Name: "testNode2",
					Uri:  "http://fake.localhost:12345",
				},
			},
			CustomVms: map[string]*rpcpb.CustomVmInfo{
				"vm1": {
					BlockchainId: "abcd",
				},
				"vm2": {
					BlockchainId: "efgh",
				},
			},
		},
	}
	c.On("Health", mock.Anything).Return(fakeHealthResponse, nil)
	c.On("Close").Return(nil)
	return c, nil
}
