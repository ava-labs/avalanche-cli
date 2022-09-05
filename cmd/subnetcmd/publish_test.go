// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTestPublisher(string, string, string) subnet.Publisher {
	mockPub := &mocks.Publisher{}
	mockPub.On("GetRepo").Return(&git.Repository{}, nil)
	mockPub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return mockPub
}

func TestNoRepoPath(t *testing.T) {
	assert, mockPrompt := setupTestEnv(t)
	defer func() {
		app = nil
		noRepoPath = ""
		forceWrite = false
	}()

	testSubnet := "testSubnet"

	configureMockPrompt(mockPrompt)

	sc := &models.Sidecar{
		VM:        models.SubnetEvm,
		VMVersion: "v0.9.99",
		Name:      testSubnet,
		Subnet:    testSubnet,
		Networks: map[string]models.NetworkData{
			models.Fuji.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
		},
	}

	// first try with an impossible file
	noRepoPath = "/path/to/nowhere"
	err := doPublish(sc, testSubnet, newTestPublisher)
	// should fail as it can't create that dir
	assert.Error(err)
	assert.ErrorContains(err, "failed")

	// try with existing files
	noRepoPath = t.TempDir()
	subnetDir := filepath.Join(noRepoPath, constants.SubnetDir)
	vmDir := filepath.Join(noRepoPath, constants.VMDir)
	err = os.MkdirAll(subnetDir, constants.DefaultPerms755)
	assert.NoError(err)
	err = os.MkdirAll(vmDir, constants.DefaultPerms755)
	assert.NoError(err)
	expectedSubnetFile := filepath.Join(subnetDir, testSubnet+constants.YAMLSuffix)
	expectedVMFile := filepath.Join(vmDir, sc.Networks["Fuji"].BlockchainID.String()+constants.YAMLSuffix)
	_, err = os.Create(expectedSubnetFile)
	assert.NoError(err)
	// For Sha256 calc we are accessing the subnet-evm binary
	// So we're just `touch`ing that file so the code finds it
	subnetDir := filepath.Join(app.GetSubnetEVMBinDir(), constants.SubnetEVMRepoName+"-"+sc.VMVersion)
	err = os.MkdirAll(subnetDir, constants.DefaultPerms755)
	assert.NoError(err)
	_, err = os.Create(filepath.Join(subnetDir, constants.SubnetEVMBin))
	assert.NoError(err)

	// should fail as no force flag
	err = doPublish(sc, testSubnet, newTestPublisher)
	assert.Error(err)
	assert.ErrorContains(err, "already exists")
	err = os.Remove(expectedSubnetFile)
	assert.NoError(err)
	_, err = os.Create(expectedVMFile)
	assert.NoError(err)

	// should fail as no force flag (other file)
	err = doPublish(sc, testSubnet, newTestPublisher)
	assert.Error(err)
	assert.ErrorContains(err, "already exists")
	err = os.Remove(expectedVMFile)
	assert.NoError(err)

	// this now should succeed and the file exist
	err = doPublish(sc, testSubnet, newTestPublisher)
	assert.NoError(err)
	assert.FileExists(expectedSubnetFile)
	assert.FileExists(expectedVMFile)
	// set force flag
	forceWrite = true
	// should also succeed and the file exist
	err = doPublish(sc, testSubnet, newTestPublisher)
	assert.NoError(err)
	assert.FileExists(expectedSubnetFile)
	assert.FileExists(expectedVMFile)

	// reset expectations as TestPublishing also uses the same mocks
	// but those are global so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
}

func TestCanPublish(t *testing.T) {
	assert, _ := setupTestEnv(t)
	defer func() {
		app = nil
	}()

	scCanPublishFuji := &models.Sidecar{
		VM:     models.SubnetEvm,
		Name:   "fuji",
		Subnet: "fuji",
		Networks: map[string]models.NetworkData{
			models.Fuji.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
		},
	}

	scCanPublishMain := &models.Sidecar{
		VM:     models.SubnetEvm,
		Name:   "main",
		Subnet: "main",
		Networks: map[string]models.NetworkData{
			models.Mainnet.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
		},
	}

	scCanPublishBoth := &models.Sidecar{
		VM:     models.SubnetEvm,
		Name:   "both",
		Subnet: "both",
		Networks: map[string]models.NetworkData{
			models.Fuji.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
			models.Mainnet.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
		},
	}

	scCanNotPublishLocal := &models.Sidecar{
		VM:     models.SubnetEvm,
		Name:   "local",
		Subnet: "local",
		Networks: map[string]models.NetworkData{
			models.Local.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
		},
	}

	scCanNotPublishUndefined := &models.Sidecar{
		VM:     models.SubnetEvm,
		Name:   "undefined",
		Subnet: "undefined",
		Networks: map[string]models.NetworkData{
			models.Undefined.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
		},
	}

	scCanNotPublishBothInvalid := &models.Sidecar{
		VM:     models.SubnetEvm,
		Name:   "bothInvalid",
		Subnet: "bothInvalid",
		Networks: map[string]models.NetworkData{
			models.Undefined.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
			models.Local.String(): {
				SubnetID:     ids.GenerateTestID(),
				BlockchainID: ids.GenerateTestID(),
			},
		},
	}

	sidecars := []*models.Sidecar{
		scCanPublishFuji,
		scCanPublishMain,
		scCanPublishBoth,
		scCanNotPublishLocal,
		scCanNotPublishUndefined,
		scCanNotPublishBothInvalid,
	}

	for i, sc := range sidecars {
		ready := isReadyToPublish(sc)
		if i < 3 {
			assert.True(ready)
		} else {
			assert.False(ready)
		}
	}
}

func TestIsPublished(t *testing.T) {
	assert, _ := setupTestEnv(t)
	defer func() {
		app = nil
	}()

	testSubnet := "testSubnet"

	published, err := isAlreadyPublished(testSubnet)
	assert.NoError(err)
	assert.False(published)

	baseDir := app.GetBaseDir()
	err = os.Mkdir(filepath.Join(baseDir, testSubnet), constants.DefaultPerms755)
	assert.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	assert.NoError(err)
	assert.False(published)

	reposDir := app.GetReposDir()
	err = os.MkdirAll(filepath.Join(reposDir, "dummyRepo", constants.VMDir, testSubnet), constants.DefaultPerms755)
	assert.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	assert.NoError(err)
	assert.False(published)

	goodDir1 := filepath.Join(reposDir, "dummyRepo", constants.SubnetDir, testSubnet)
	err = os.MkdirAll(goodDir1, constants.DefaultPerms755)
	assert.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	assert.NoError(err)
	assert.False(published)

	_, err = os.Create(filepath.Join(goodDir1, testSubnet))
	assert.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	assert.NoError(err)
	assert.True(published)

	goodDir2 := filepath.Join(reposDir, "dummyRepo2", constants.SubnetDir, testSubnet)
	err = os.MkdirAll(goodDir2, constants.DefaultPerms755)
	assert.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	assert.NoError(err)
	assert.True(published)
	_, err = os.Create(filepath.Join(goodDir2, "myOtherTestSubnet"))
	assert.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	assert.NoError(err)
	assert.True(published)

	_, err = os.Create(filepath.Join(goodDir2, testSubnet))
	assert.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	assert.NoError(err)
	assert.True(published)
}

// TestPublishing allows unit testing of the **normal** flow for publishing
func TestPublishing(t *testing.T) {
	assert, mockPrompt := setupTestEnv(t)
	defer func() {
		app = nil
	}()

	configureMockPrompt(mockPrompt)

	sc := &models.Sidecar{
		VM:        models.SubnetEvm,
		VMVersion: "v0.9.99",
	}
	// For Sha256 calc we are accessing the subnet-evm binary
	// So we're just `touch`ing that file so the code finds it
	subnetDir := filepath.Join(app.GetSubnetEVMBinDir(), constants.SubnetEVMRepoName+"-"+sc.VMVersion)
	err := os.MkdirAll(subnetDir, constants.DefaultPerms755)
	assert.NoError(err)
	_, err = os.Create(filepath.Join(subnetDir, constants.SubnetEVMBin))
	assert.NoError(err)

	err = doPublish(sc, "testSubnet", newTestPublisher)
	assert.NoError(err)

	// reset expectations as TestNoRepoPath also uses the same mocks
	// but those are global so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
}

func configureMockPrompt(mockPrompt *mocks.Prompter) {
	// capture string for a repo alias...
	mockPrompt.On("CaptureString", mock.Anything).Return("testAlias", nil).Once()
	// then the repo URL...
	mockPrompt.On("CaptureString", mock.Anything).Return("https://localhost:12345", nil).Once()
	// always provide an irrelevant response when empty is allowed...
	mockPrompt.On("CaptureStringAllowEmpty", mock.Anything).Return("irrelevant", nil)
	// on the maintainers, return some array
	mockPrompt.On("CaptureListDecision", mockPrompt, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]any{"dummy", "stuff"}, false, nil)
	// finally return a semantic version
	mockPrompt.On("CaptureVersion", mock.Anything).Return("v0.9.99", nil)
}

func setupTestEnv(t *testing.T) (*assert.Assertions, *mocks.Prompter) {
	assert := assert.New(t)
	testDir := t.TempDir()
	err := os.Mkdir(filepath.Join(testDir, "repos"), 0o755)
	assert.NoError(err)
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	app = &application.Avalanche{}
	mockPrompt := mocks.NewPrompter(t)
	app.Setup(testDir, logging.NoLog{}, config.New(), mockPrompt)

	return assert, mockPrompt
}
