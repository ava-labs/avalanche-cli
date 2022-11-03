// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/version"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testSubnet = "testSubnet"
)

func TestInfoKnownVMs(t *testing.T) {
	assert := assert.New(t)
	vmBinDir := t.TempDir()
	expectedSHA := "a12871fee210fb8619291eaea194581cbd2531e4b23759d225f6806923f63222"

	type testCase struct {
		strVer   string
		repoName string
		vmBinDir string
		vmBin    string
		dl       binutils.GithubDownloader
	}

	cases := []testCase{
		{
			strVer:   "v0.9.99",
			repoName: "spacesvm",
			vmBinDir: vmBinDir,
			vmBin:    "mySpacesVM",
			dl:       binutils.NewSpacesVMDownloader(),
		},
		{
			strVer:   "v0.9.99",
			repoName: "subnet-evm",
			vmBinDir: vmBinDir,
			vmBin:    "mySubnetEVM",
			dl:       binutils.NewSubnetEVMDownloader(),
		},
	}

	for _, c := range cases {
		binDir := filepath.Join(vmBinDir, c.repoName+"-"+c.strVer)
		err := os.MkdirAll(binDir, constants.DefaultPerms755)
		assert.NoError(err)
		err = os.WriteFile(filepath.Join(binDir, c.vmBin), []byte{0x1, 0x2}, constants.DefaultPerms755)
		assert.NoError(err)
		maintrs, ver, resurl, sha, err := getInfoForKnownVMs(
			c.strVer,
			c.repoName,
			c.vmBinDir,
			c.vmBin,
			c.dl,
		)
		assert.NoError(err)
		assert.ElementsMatch([]string{constants.AvaLabsMaintainers}, maintrs)
		assert.NoError(err)
		_, err = url.Parse(resurl)
		assert.NoError(err)
		// it's kinda useless to create the URL by building it via downloader -
		// would defeat the purpose of the test
		expectedURL := "https://github.com/ava-labs/" +
			c.repoName + "/releases/download/" +
			c.strVer + "/" + c.repoName + "_" + c.strVer[1:] + "_" +
			runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
		assert.Equal(expectedURL, resurl)
		assert.Equal(&version.Semantic{
			Major: 0,
			Minor: 9,
			Patch: 99,
		}, ver)
		assert.Equal(expectedSHA, sha)
	}
}

func TestNoRepoPath(t *testing.T) {
	assert, mockPrompt := setupTestEnv(t)
	defer func() {
		app = nil
		noRepoPath = ""
		forceWrite = false
	}()

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
	appSubnetDir := filepath.Join(app.GetSubnetEVMBinDir(), constants.SubnetEVMRepoName+"-"+sc.VMVersion)
	err = os.MkdirAll(appSubnetDir, constants.DefaultPerms755)
	assert.NoError(err)
	_, err = os.Create(filepath.Join(appSubnetDir, constants.SubnetEVMBin))
	assert.NoError(err)

	// reset expectations as this test (and TestPublishing) also uses the same mocks
	// and the same sequence so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
	configureMockPrompt(mockPrompt)

	// should fail as no force flag
	err = doPublish(sc, testSubnet, newTestPublisher)
	assert.Error(err)
	assert.ErrorContains(err, "already exists")
	err = os.Remove(expectedSubnetFile)
	assert.NoError(err)
	_, err = os.Create(expectedVMFile)
	assert.NoError(err)

	// next should fail as no force flag (other file)

	// reset expectations as this test (and TestPublishing) also uses the same mocks
	// and the same sequence so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
	configureMockPrompt(mockPrompt)

	err = doPublish(sc, testSubnet, newTestPublisher)
	assert.Error(err)
	assert.ErrorContains(err, "already exists")
	err = os.Remove(expectedVMFile)
	assert.NoError(err)

	// this now should succeed and the file exist

	// reset expectations as this test (and TestPublishing) also uses the same mocks
	// and the same sequence so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
	configureMockPrompt(mockPrompt)

	err = doPublish(sc, testSubnet, newTestPublisher)
	assert.NoError(err)
	assert.FileExists(expectedSubnetFile)
	assert.FileExists(expectedVMFile)

	// set force flag
	forceWrite = true

	// should also succeed and the file exist

	// reset expectations as this test (and TestPublishing) also uses the same mocks
	// and the same sequence so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
	configureMockPrompt(mockPrompt)

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

	err = doPublish(sc, testSubnet, newTestPublisher)
	assert.NoError(err)

	// reset expectations as TestNoRepoPath also uses the same mocks
	// but those are global so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
}

func configureMockPrompt(mockPrompt *mocks.Prompter) {
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return("Add", nil).Once()
	mockPrompt.On("CaptureEmail", mock.Anything).Return("someone@somewhere.com", nil)
	mockPrompt.On("CaptureList", mock.Anything, mock.Anything).Return("Done", nil).Once()
	// capture string for a repo alias...
	mockPrompt.On("CaptureString", mock.Anything).Return("testAlias", nil).Once()
	// then the repo URL...
	mockPrompt.On("CaptureString", mock.Anything).Return("https://localhost:12345", nil).Once()
	// always provide an irrelevant response when empty is allowed...
	mockPrompt.On("CaptureStringAllowEmpty", mock.Anything).Return("irrelevant", nil)
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
	app.Setup(testDir, logging.NoLog{}, config.New(), mockPrompt, application.NewDownloader())

	return assert, mockPrompt
}

func newTestPublisher(string, string, string) subnet.Publisher {
	mockPub := &mocks.Publisher{}
	mockPub.On("GetRepo").Return(&git.Repository{}, nil)
	mockPub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return mockPub
}
