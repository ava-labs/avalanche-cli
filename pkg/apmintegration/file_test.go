// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package apmintegration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/apm/types"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

const (
	org1 = "org1"
	org2 = "org2"

	repo1 = "repo1"
	repo2 = "repo2"

	subnet1 = "testsubnet1"
	subnet2 = "testsubnet2"

	vm = "testvm"

	testSubnetYaml = `subnet:
  id:
    k1: v1
    k2: v2
  alias: "testsubnet"
  homepage: "https://subnet.com"
  description: It's a subnet
  maintainers:
    - "dev@subnet.com"
  vms:
    - "testvm1"
    - "testvm2"
`

	testVMYaml = `vm:
  id: "efgh"
  alias: "testvm"
  homepage: "https://vm.com"
  description: "Virtual machine"
  maintainers:
    - "dev@vm.com"
  installScript: "scripts/build.sh"
  binaryPath: "build/sqja3uK17MJxfC7AN8nGadBw9JK5BcrsNwNynsqP5Gih8M5Bm"
  url: "https://github.com/org/repo/archive/refs/tags/v1.0.0.tar.gz"
  sha256: "1ac250f6c40472f22eaf0616fc8c886078a4eaa9b2b85fbb4fb7783a1db6af3f"
  version:
    major: 1
    minor: 0
    patch: 0
`
)

func newTestApp(t *testing.T, testDir string) *application.Avalanche {
	tempDir := t.TempDir()
	app := application.New()
	app.Setup(tempDir, logging.NoLog{}, nil, prompts.NewPrompter(), application.NewDownloader())
	app.ApmDir = testDir
	return app
}

func TestGetRepos(t *testing.T) {
	type test struct {
		name  string
		orgs  []string
		repos []string
	}

	tests := []test{
		{
			name:  "Single",
			orgs:  []string{org1},
			repos: []string{repo1},
		},
		{
			name:  "Multiple",
			orgs:  []string{org1, org2},
			repos: []string{repo1, repo2},
		},
		{
			name:  "Empty",
			orgs:  []string{},
			repos: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			testDir := t.TempDir()
			app := newTestApp(t, testDir)

			repositoryDir := filepath.Join(testDir, "repositories")
			err := os.Mkdir(repositoryDir, constants.DefaultPerms755)
			require.NoError(err)

			// create repos
			for _, org := range tt.orgs {
				for _, repo := range tt.repos {
					repoPath := filepath.Join(repositoryDir, org, repo)
					err = os.MkdirAll(repoPath, constants.DefaultPerms755)
					require.NoError(err)
				}
			}

			// test function
			repos, err := GetRepos(app)
			require.NoError(err)

			// check results
			numRepos := len(tt.orgs) * len(tt.repos)
			require.Equal(numRepos, len(repos))

			index := 0
			for _, org := range tt.orgs {
				for _, repo := range tt.repos {
					require.Equal(org+"/"+repo, repos[index])
					index++
				}
			}
		})
	}
}

func TestGetSubnets(t *testing.T) {
	type test struct {
		name        string
		org         string
		repo        string
		subnetNames []string
	}

	tests := []test{
		{
			name:        "Single",
			org:         org1,
			repo:        repo1,
			subnetNames: []string{subnet1},
		},
		{
			name:        "Multiple",
			org:         org1,
			repo:        repo1,
			subnetNames: []string{subnet1, subnet2},
		},
		{
			name:        "Empty",
			org:         org1,
			repo:        repo1,
			subnetNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			testDir := t.TempDir()
			app := newTestApp(t, testDir)

			// Setup subnet directory
			subnetPath := filepath.Join(testDir, "repositories", tt.org, tt.repo, "subnets")
			err := os.MkdirAll(subnetPath, constants.DefaultPerms755)
			require.NoError(err)

			// Create subnet files
			for _, subnet := range tt.subnetNames {
				subnetFile := filepath.Join(subnetPath, subnet+yamlExt)
				err = os.WriteFile(subnetFile, []byte(testSubnetYaml), constants.DefaultPerms755)
				require.NoError(err)
			}

			subnets, err := GetSubnets(app, makeAlias(tt.org, tt.repo))
			require.NoError(err)

			// check results
			require.Equal(len(tt.subnetNames), len(subnets))
			for i, subnet := range tt.subnetNames {
				require.Equal(tt.subnetNames[i], subnet)
			}
		})
	}
}

func TestLoadSubnetFile_Success(t *testing.T) {
	require := require.New(t)

	testDir := t.TempDir()
	app := newTestApp(t, testDir)

	// Setup subnet directory
	subnetPath := filepath.Join(testDir, "repositories", org1, repo1, "subnets")
	err := os.MkdirAll(subnetPath, constants.DefaultPerms755)
	require.NoError(err)

	// Create subnet files
	subnetFile := filepath.Join(subnetPath, subnet1+yamlExt)
	err = os.WriteFile(subnetFile, []byte(testSubnetYaml), constants.DefaultPerms755)
	require.NoError(err)

	expectedSubnet := types.Subnet{
		ID: map[string]string{
			"k1": "v1",
			"k2": "v2",
		},
		Alias:       "testsubnet",
		Homepage:    "https://subnet.com",
		Description: "It's a subnet",
		Maintainers: []string{"dev@subnet.com"},
		VMs:         []string{"testvm1", "testvm2"},
	}

	loadedSubnet, err := LoadSubnetFile(app, MakeKey(makeAlias(org1, repo1), subnet1))
	require.NoError(err)
	require.Equal(expectedSubnet, loadedSubnet)
}

func TestLoadSubnetFile_BadKey(t *testing.T) {
	require := require.New(t)

	testDir := t.TempDir()
	app := newTestApp(t, testDir)

	// Setup subnet directory
	subnetPath := filepath.Join(testDir, "repositories", org1, repo1, "subnets")
	err := os.MkdirAll(subnetPath, constants.DefaultPerms755)
	require.NoError(err)

	// Create subnet files
	subnetFile := filepath.Join(subnetPath, subnet1+yamlExt)
	err = os.WriteFile(subnetFile, []byte(testSubnetYaml), constants.DefaultPerms755)
	require.NoError(err)

	_, err = LoadSubnetFile(app, subnet1)
	require.ErrorContains(err, "invalid subnet key")
}

func TestGetVMsInSubnet(t *testing.T) {
	require := require.New(t)

	testDir := t.TempDir()
	app := newTestApp(t, testDir)

	// Setup subnet directory
	subnetPath := filepath.Join(testDir, "repositories", org1, repo1, "subnets")
	err := os.MkdirAll(subnetPath, constants.DefaultPerms755)
	require.NoError(err)

	// Create subnet files
	subnetFile := filepath.Join(subnetPath, subnet1+yamlExt)
	err = os.WriteFile(subnetFile, []byte(testSubnetYaml), constants.DefaultPerms755)
	require.NoError(err)

	expectedVMs := []string{"testvm1", "testvm2"}

	loadedVMs, err := getVMsInSubnet(app, MakeKey(makeAlias(org1, repo1), subnet1))
	require.NoError(err)
	require.Equal(expectedVMs, loadedVMs)
}

func TestLoadVMFile(t *testing.T) {
	require := require.New(t)

	testDir := t.TempDir()
	app := newTestApp(t, testDir)

	// Setup vm directory
	vmPath := filepath.Join(testDir, "repositories", org1, repo1, "vms")
	err := os.MkdirAll(vmPath, constants.DefaultPerms755)
	require.NoError(err)

	// Create subnet files
	vmFile := filepath.Join(vmPath, vm+yamlExt)
	err = os.WriteFile(vmFile, []byte(testVMYaml), constants.DefaultPerms755)
	require.NoError(err)

	expectedVM := types.VM{
		ID:            "efgh",
		Alias:         vm,
		Homepage:      "https://vm.com",
		Description:   "Virtual machine",
		Maintainers:   []string{"dev@vm.com"},
		BinaryPath:    "build/sqja3uK17MJxfC7AN8nGadBw9JK5BcrsNwNynsqP5Gih8M5Bm",
		InstallScript: "scripts/build.sh",
		URL:           "https://github.com/org/repo/archive/refs/tags/v1.0.0.tar.gz",
		SHA256:        "1ac250f6c40472f22eaf0616fc8c886078a4eaa9b2b85fbb4fb7783a1db6af3f",
	}

	loadedVM, err := LoadVMFile(app, makeAlias(org1, repo1), vm)
	require.NoError(err)
	require.Equal(expectedVM, loadedVM)
}
