// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/proto"
)

var (
	testBlockChainID1 = ids.GenerateTestID().String()
	testBlockChainID2 = ids.GenerateTestID().String()
	testSubnetID1     = ids.GenerateTestID().String()
	testSubnetID2     = ids.GenerateTestID().String()

	testVMID   = "tGBrM2SXkAdNsqzb3SaFZZWMNdzjjFEUKteheTa4dhUwnfQyu" // VM ID of "test"
	testChainName = "test"

	fakeHealthResponse = &rpcpb.HealthResponse{
		ClusterInfo: &rpcpb.ClusterInfo{
			Healthy:             true, // currently actually not checked, should it, if CustomVMsHealthy already is?
			CustomChainsHealthy: true,
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
			CustomChains: map[string]*rpcpb.CustomChainInfo{
				"bchain1": {
					ChainId: testBlockChainID1,
				},
				"bchain2": {
					ChainId: testBlockChainID2,
				},
			},
			Subnets: []string{testSubnetID1, testSubnetID2},
		},
	}
)

func setupTest(t *testing.T) *assert.Assertions {
	// use io.Discard to not print anything
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	return assert.New(t)
}

func TestDeployToLocal(t *testing.T) {
	assert := setupTest(t)

	// fake-return true simulating the process is running
	procChecker := &mocks.ProcessChecker{}
	procChecker.On("IsServerProcessRunning", mock.Anything).Return(true, nil)

	// create a dummy plugins dir, deploy will check it exists
	binChecker := &mocks.BinaryChecker{}
	tmpDir := t.TempDir()
	err := os.Mkdir(filepath.Join(tmpDir, "plugins"), perms.ReadWriteExecute)
	assert.NoError(err)

	// create a dummy avalanchego file, deploy will check it exists
	f, err := os.Create(filepath.Join(tmpDir, "avalanchego"))
	assert.NoError(err)
	defer func() {
		_ = f.Close()
	}()

	binChecker.On("ExistsWithLatestVersion", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(true, tmpDir, nil)

	binDownloader := &mocks.PluginBinaryDownloader{}
	binDownloader.On("Download", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	app := &application.Avalanche{
		Log: logging.NoLog{},
	}

	testDeployer := &LocalSubnetDeployer{
		procChecker:         procChecker,
		binChecker:          binChecker,
		getClientFunc:       getTestClientFunc,
		binaryDownloader:    binDownloader,
		healthCheckInterval: 500 * time.Millisecond,
		app:                 app,
		setDefaultSnapshot:  fakeSetDefaultSnapshot,
	}

	// create a simple genesis for the test
	genesis := `{"config":{"chainId":9999},"gasLimit":"0x0","difficulty":"0x0","alloc":{}}`
	// create a dummy genesis file, deploy will check it exists
	testGenesis, err := os.CreateTemp(tmpDir, "test-genesis.json")
	assert.NoError(err)

	err = os.WriteFile(testGenesis.Name(), []byte(genesis), constants.DefaultPerms755)
	assert.NoError(err)
	// test actual deploy
	s, b, err := testDeployer.DeployToLocalNetwork(testChainName, []byte(genesis), testGenesis.Name())
	assert.NoError(err)
	assert.Equal(testSubnetID2, s.String())
	assert.Equal(testBlockChainID2, b.String())
}

func TestExistsWithLatestVersion(t *testing.T) {
	assert := setupTest(t)

	tmpDir := t.TempDir()
	bc := binutils.NewBinaryChecker()

	exists, latest, err := bc.ExistsWithLatestVersion(tmpDir, "avalanchego-v")
	assert.NoError(err)
	assert.False(exists)
	assert.Empty(latest)

	fake := filepath.Join(tmpDir, "anything")
	err = os.Mkdir(fake, perms.ReadWriteExecute)
	assert.NoError(err)
	exists, latest, err = bc.ExistsWithLatestVersion(tmpDir, "avalanchego-v")
	assert.NoError(err)
	assert.False(exists)
	assert.Empty(latest)

	avagoOnly := filepath.Join(tmpDir, "avalanchego")
	err = os.Mkdir(avagoOnly, perms.ReadWriteExecute)
	assert.NoError(err)
	exists, latest, err = bc.ExistsWithLatestVersion(tmpDir, "avalanchego-v")
	assert.NoError(err)
	assert.False(exists)
	assert.Empty(latest)

	existsOneOnly := filepath.Join(tmpDir, "avalanchego-v1.7.10")
	err = os.Mkdir(existsOneOnly, perms.ReadWriteExecute)
	assert.NoError(err)
	exists, latest, err = bc.ExistsWithLatestVersion(tmpDir, "avalanchego-v")
	assert.NoError(err)
	assert.True(exists)
	assert.Equal(existsOneOnly, latest)

	ver1 := filepath.Join(tmpDir, "avalanchego-v1.8.0")
	ver2 := filepath.Join(tmpDir, "avalanchego-v1.18.0")
	ver3 := filepath.Join(tmpDir, "avalanchego-v1.8.1")
	ver4 := filepath.Join(tmpDir, "avalanchego-v0.8.0")
	ver5 := filepath.Join(tmpDir, "avalanchego-v0.88.0")
	ver6 := filepath.Join(tmpDir, "avalanchego-v0.1.0")
	ver7 := filepath.Join(tmpDir, "avalanchego-v0.11.0")
	ver8 := filepath.Join(tmpDir, "avalanchego-0.11.0")

	err = os.Mkdir(ver1, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver2, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver3, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver4, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver5, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver6, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver7, perms.ReadWriteExecute)
	assert.NoError(err)
	err = os.Mkdir(ver8, perms.ReadWriteExecute)
	assert.NoError(err)

	exists, latest, err = bc.ExistsWithLatestVersion(tmpDir, "avalanchego-v")
	assert.NoError(err)
	assert.True(exists)
	assert.Equal(ver2, latest)
}

func TestGetLatestAvagoVersion(t *testing.T) {
	assert := setupTest(t)

	testVersion := "v1.99.9999"
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := fmt.Sprintf(`{"some":"unimportant","fake":"data","tag_name":"%s","tag_name_was":"what we are interested in"}`, testVersion)
		_, err := w.Write([]byte(resp))
		assert.NoError(err)
	})
	s := httptest.NewServer(testHandler)
	defer s.Close()

	v, err := binutils.GetLatestReleaseVersion(s.URL)
	assert.NoError(err)
	assert.Equal(v, testVersion)
}

func getTestClientFunc() (client.Client, error) {
	c := &mocks.Client{}
	fakeLoadSnapshotResponse := &rpcpb.LoadSnapshotResponse{}
	fakeSaveSnapshotResponse := &rpcpb.SaveSnapshotResponse{}
	fakeRemoveSnapshotResponse := &rpcpb.RemoveSnapshotResponse{}
	fakeCreateBlockchainsResponse := &rpcpb.CreateBlockchainsResponse{}
	c.On("LoadSnapshot", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fakeLoadSnapshotResponse, nil)
	c.On("SaveSnapshot", mock.Anything, mock.Anything).Return(fakeSaveSnapshotResponse, nil)
	c.On("RemoveSnapshot", mock.Anything, mock.Anything).Return(fakeRemoveSnapshotResponse, nil)
	c.On("CreateBlockchains", mock.Anything, mock.Anything, mock.Anything).Return(fakeCreateBlockchainsResponse, nil)
	c.On("URIs", mock.Anything).Return([]string{"fakeUri"}, nil)
	// When fake deploying, the first response needs to have a bogus subnet ID, because
	// otherwise the doDeploy function "aborts" when checking if the subnet had already been deployed.
	// Afterwards, we can set the actual VM ID so that the test returns an expected subnet ID...

	// Return a fake health response twice
	c.On("Health", mock.Anything).Return(fakeHealthResponse, nil).Twice()
	// Afterwards, change the VmId so that TestDeployToLocal has the correct ID to check
	alteredFakeResponse := proto.Clone(fakeHealthResponse).(*rpcpb.HealthResponse) // new(rpcpb.HealthResponse)
	alteredFakeResponse.ClusterInfo.CustomChains["bchain2"].VmId = testVMID
	alteredFakeResponse.ClusterInfo.CustomChains["bchain2"].ChainName = testChainName
	alteredFakeResponse.ClusterInfo.CustomChains["bchain1"].ChainName = "bchain1"
	c.On("Health", mock.Anything).Return(alteredFakeResponse, nil)
	c.On("Close").Return(nil)
	return c, nil
}

func fakeSetDefaultSnapshot(baseDir string, force bool) error {
	return nil
}
