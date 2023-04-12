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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testSubnet = "testSubnet"
)

func TestInfoKnownVMs(t *testing.T) {
	require := require.New(t)
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
			repoName: "subnet-evm",
			vmBinDir: vmBinDir,
			vmBin:    "mySubnetEVM",
			dl:       binutils.NewSubnetEVMDownloader(),
		},
	}

	for _, c := range cases {
		binDir := filepath.Join(vmBinDir, c.repoName+"-"+c.strVer)
		err := os.MkdirAll(binDir, constants.DefaultPerms755)
		require.NoError(err)
		err = os.WriteFile(filepath.Join(binDir, c.vmBin), []byte{0x1, 0x2}, constants.DefaultPerms755)
		require.NoError(err)
		maintrs, ver, resurl, sha, err := getInfoForKnownVMs(
			c.strVer,
			c.repoName,
			c.vmBinDir,
			c.vmBin,
			c.dl,
		)
		require.NoError(err)
		require.ElementsMatch([]string{constants.AvaLabsMaintainers}, maintrs)
		require.NoError(err)
		_, err = url.Parse(resurl)
		require.NoError(err)
		// it's kinda useless to create the URL by building it via downloader -
		// would defeat the purpose of the test
		expectedURL := "https://github.com/ava-labs/" +
			c.repoName + "/releases/download/" +
			c.strVer + "/" + c.repoName + "_" + c.strVer[1:] + "_" +
			runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
		require.Equal(expectedURL, resurl)
		require.Equal(&version.Semantic{
			Major: 0,
			Minor: 9,
			Patch: 99,
		}, ver)
		require.Equal(expectedSHA, sha)
	}
}

func TestNoRepoPath(t *testing.T) {
	require, mockPrompt := setupTestEnv(t)
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
	require.Error(err)
	require.ErrorContains(err, "failed")

	// try with existing files
	noRepoPath = t.TempDir()
	subnetDir := filepath.Join(noRepoPath, constants.SubnetDir)
	vmDir := filepath.Join(noRepoPath, constants.VMDir)
	err = os.MkdirAll(subnetDir, constants.DefaultPerms755)
	require.NoError(err)
	err = os.MkdirAll(vmDir, constants.DefaultPerms755)
	require.NoError(err)
	expectedSubnetFile := filepath.Join(subnetDir, testSubnet+constants.YAMLSuffix)
	expectedVMFile := filepath.Join(vmDir, sc.Networks["Fuji"].BlockchainID.String()+constants.YAMLSuffix)
	_, err = os.Create(expectedSubnetFile)
	require.NoError(err)

	// For Sha256 calc we are accessing the subnet-evm binary
	// So we're just `touch`ing that file so the code finds it
	appSubnetDir := filepath.Join(app.GetSubnetEVMBinDir(), constants.SubnetEVMRepoName+"-"+sc.VMVersion)
	err = os.MkdirAll(appSubnetDir, constants.DefaultPerms755)
	require.NoError(err)
	_, err = os.Create(filepath.Join(appSubnetDir, constants.SubnetEVMBin))
	require.NoError(err)

	// reset expectations as this test (and TestPublishing) also uses the same mocks
	// and the same sequence so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
	configureMockPrompt(mockPrompt)

	// should fail as no force flag
	err = doPublish(sc, testSubnet, newTestPublisher)
	require.Error(err)
	require.ErrorContains(err, "already exists")
	err = os.Remove(expectedSubnetFile)
	require.NoError(err)
	_, err = os.Create(expectedVMFile)
	require.NoError(err)

	// next should fail as no force flag (other file)

	// reset expectations as this test (and TestPublishing) also uses the same mocks
	// and the same sequence so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
	configureMockPrompt(mockPrompt)

	err = doPublish(sc, testSubnet, newTestPublisher)
	require.Error(err)
	require.ErrorContains(err, "already exists")
	err = os.Remove(expectedVMFile)
	require.NoError(err)

	// this now should succeed and the file exist

	// reset expectations as this test (and TestPublishing) also uses the same mocks
	// and the same sequence so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
	configureMockPrompt(mockPrompt)

	err = doPublish(sc, testSubnet, newTestPublisher)
	require.NoError(err)
	require.FileExists(expectedSubnetFile)
	require.FileExists(expectedVMFile)

	// set force flag
	forceWrite = true

	// should also succeed and the file exist

	// reset expectations as this test (and TestPublishing) also uses the same mocks
	// and the same sequence so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
	configureMockPrompt(mockPrompt)

	err = doPublish(sc, testSubnet, newTestPublisher)
	require.NoError(err)
	require.FileExists(expectedSubnetFile)
	require.FileExists(expectedVMFile)

	// reset expectations as TestPublishing also uses the same mocks
	// but those are global so expectations get messed up
	mockPrompt.Calls = nil
	mockPrompt.ExpectedCalls = nil
}

func TestCanPublish(t *testing.T) {
	require, _ := setupTestEnv(t)
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
			require.True(ready)
		} else {
			require.False(ready)
		}
	}
}

func TestIsPublished(t *testing.T) {
	require, _ := setupTestEnv(t)
	defer func() {
		app = nil
	}()

	published, err := isAlreadyPublished(testSubnet)
	require.NoError(err)
	require.False(published)

	baseDir := app.GetBaseDir()
	err = os.Mkdir(filepath.Join(baseDir, testSubnet), constants.DefaultPerms755)
	require.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	require.NoError(err)
	require.False(published)

	reposDir := app.GetReposDir()
	err = os.MkdirAll(filepath.Join(reposDir, "dummyRepo", constants.VMDir, testSubnet), constants.DefaultPerms755)
	require.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	require.NoError(err)
	require.False(published)

	goodDir1 := filepath.Join(reposDir, "dummyRepo", constants.SubnetDir, testSubnet)
	err = os.MkdirAll(goodDir1, constants.DefaultPerms755)
	require.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	require.NoError(err)
	require.False(published)

	_, err = os.Create(filepath.Join(goodDir1, testSubnet))
	require.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	require.NoError(err)
	require.True(published)

	goodDir2 := filepath.Join(reposDir, "dummyRepo2", constants.SubnetDir, testSubnet)
	err = os.MkdirAll(goodDir2, constants.DefaultPerms755)
	require.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	require.NoError(err)
	require.True(published)
	_, err = os.Create(filepath.Join(goodDir2, "myOtherTestSubnet"))
	require.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	require.NoError(err)
	require.True(published)

	_, err = os.Create(filepath.Join(goodDir2, testSubnet))
	require.NoError(err)
	published, err = isAlreadyPublished(testSubnet)
	require.NoError(err)
	require.True(published)
}

// TestPublishing allows unit testing of the **normal** flow for publishing
func TestPublishing(t *testing.T) {
	require, mockPrompt := setupTestEnv(t)
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
	require.NoError(err)
	_, err = os.Create(filepath.Join(subnetDir, constants.SubnetEVMBin))
	require.NoError(err)

	err = doPublish(sc, testSubnet, newTestPublisher)
	require.NoError(err)

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

func setupTestEnv(t *testing.T) (*require.Assertions, *mocks.Prompter) {
	require := require.New(t)
	testDir := t.TempDir()
	err := os.Mkdir(filepath.Join(testDir, "repos"), 0o755)
	require.NoError(err)
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	app = &application.Avalanche{}
	mockPrompt := mocks.NewPrompter(t)
	app.Setup(testDir, logging.NoLog{}, config.New(), mockPrompt, application.NewDownloader())

	return require, mockPrompt
}

func newTestPublisher(string, string, string) subnet.Publisher {
	mockPub := &mocks.Publisher{}
	mockPub.On("GetRepo").Return(&git.Repository{}, nil)
	mockPub.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return mockPub
}
