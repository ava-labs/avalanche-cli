// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package application

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/stretchr/testify/require"
)

const (
	subnetName1 = "TEST_subnet"
	subnetName2 = "TEST_copied_subnet"
)

func TestUpdateSideCar(t *testing.T) {
	require := require.New(t)
	sc := &models.Sidecar{
		Name:      "TEST",
		VM:        models.SubnetEvm,
		TokenName: "TEST",
		ChainID:   "42",
	}

	ap := newTestApp(t)

	err := ap.CreateSidecar(sc)
	require.NoError(err)
	control, err := ap.LoadSidecar(sc.Name)
	require.NoError(err)
	require.Equal(*sc, control)
	sc.Networks = make(map[string]models.NetworkData)
	sc.Networks["local"] = models.NetworkData{
		BlockchainID: ids.GenerateTestID(),
		SubnetID:     ids.GenerateTestID(),
	}

	err = ap.UpdateSidecar(sc)
	require.NoError(err)
	control, err = ap.LoadSidecar(sc.Name)
	require.NoError(err)
	require.Equal(*sc, control)
}

func Test_writeGenesisFile_success(t *testing.T) {
	require := require.New(t)
	genesisBytes := []byte("genesis")
	genesisFile := constants.GenesisFileName

	ap := newTestApp(t)
	// Write genesis
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	require.NoError(err)

	// Check file exists
	createdPath := filepath.Join(ap.GetSubnetDir(), subnetName1, genesisFile)
	_, err = os.Stat(createdPath)
	require.NoError(err)

	// Cleanup file
	err = os.Remove(createdPath)
	require.NoError(err)
}

func Test_copyGenesisFile_success(t *testing.T) {
	require := require.New(t)
	genesisBytes := []byte("genesis")

	ap := newTestApp(t)
	// Create original genesis
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	require.NoError(err)

	// Copy genesis
	createdGenesis := ap.GetGenesisPath(subnetName1)
	err = ap.CopyGenesisFile(createdGenesis, subnetName2)
	require.NoError(err)

	// Check copied file exists
	copiedGenesis := ap.GetGenesisPath(subnetName2)
	_, err = os.Stat(copiedGenesis)
	require.NoError(err)

	// Cleanup files
	err = os.Remove(createdGenesis)
	require.NoError(err)
	err = os.Remove(copiedGenesis)
	require.NoError(err)
}

func Test_copyGenesisFile_failure(t *testing.T) {
	require := require.New(t)
	// copy genesis that doesn't exist

	ap := newTestApp(t)
	// Copy genesis
	createdGenesis := ap.GetGenesisPath(subnetName1)
	err := ap.CopyGenesisFile(createdGenesis, subnetName2)
	require.Error(err)

	// Check no copied file exists
	copiedGenesis := ap.GetGenesisPath(subnetName2)
	_, err = os.Stat(copiedGenesis)
	require.Error(err)
}

func Test_createSidecar_success(t *testing.T) {
	type test struct {
		name              string
		subnetName        string
		tokenName         string
		expectedTokenName string
		chainID           string
	}

	tests := []test{
		{
			name:              "Success",
			subnetName:        subnetName1,
			tokenName:         "TOKEN",
			expectedTokenName: "TOKEN",
			chainID:           "999",
		},
		{
			name:              "no token name",
			subnetName:        subnetName1,
			tokenName:         "",
			expectedTokenName: "TEST",
			chainID:           "888",
		},
	}

	ap := newTestApp(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			const vm = models.SubnetEvm

			sc := &models.Sidecar{
				Name:      tt.subnetName,
				VM:        vm,
				TokenName: tt.tokenName,
				ChainID:   tt.chainID,
			}

			// Write sidecar
			err := ap.CreateSidecar(sc)
			require.NoError(err)

			// Check file exists
			createdPath := ap.GetSidecarPath(tt.subnetName)
			_, err = os.Stat(createdPath)
			require.NoError(err)

			control, err := ap.LoadSidecar(tt.subnetName)
			require.NoError(err)
			require.Equal(*sc, control)

			require.Equal(sc.TokenName, tt.expectedTokenName)

			// Cleanup file
			err = os.Remove(createdPath)
			require.NoError(err)
		})
	}
}

func Test_loadSidecar_success(t *testing.T) {
	require := require.New(t)
	const vm = models.SubnetEvm

	ap := newTestApp(t)

	// Write sidecar
	sidecarBytes := []byte("{  \"Name\": \"TEST_subnet\",\n  \"VM\": \"Subnet-EVM\",\n  \"Subnet\": \"TEST_subnet\"\n  }")
	sidecarPath := ap.GetSidecarPath(subnetName1)
	err := os.MkdirAll(filepath.Dir(sidecarPath), constants.DefaultPerms755)
	require.NoError(err)

	err = os.WriteFile(sidecarPath, sidecarBytes, 0o600)
	require.NoError(err)

	// Check file exists
	_, err = os.Stat(sidecarPath)
	require.NoError(err)

	// Check contents
	expectedSc := models.Sidecar{
		Name:      subnetName1,
		VM:        vm,
		Subnet:    subnetName1,
		TokenName: constants.DefaultTokenName,
	}

	sc, err := ap.LoadSidecar(subnetName1)
	require.NoError(err)
	require.Equal(sc, expectedSc)

	// Cleanup file
	err = os.Remove(sidecarPath)
	require.NoError(err)
}

func TestChainIDExists(t *testing.T) {
	require := require.New(t)

	sc1 := &models.Sidecar{
		Name:      "sc1",
		VM:        models.SubnetEvm,
		TokenName: "TEST",
	}

	sc2 := &models.Sidecar{
		Name:      "sc2",
		VM:        models.SubnetEvm,
		TokenName: "TEST",
	}

	type test struct {
		testName    string
		shouldExist bool
		sidecarIDs  []string
		genesisIDs  []int64
		sidecars    []*models.Sidecar
	}

	ap := newTestApp(t)

	tests := []test{
		{
			testName:    "no sidecars",
			sidecars:    []*models.Sidecar{},
			shouldExist: false,
		},
		{
			testName:    "old sidecars without chain IDs only genesis all different",
			sidecars:    []*models.Sidecar{sc1, sc2},
			genesisIDs:  []int64{88, 99},
			shouldExist: false,
		},
		{
			testName:    "old sidecars without chain IDs only genesis one exists",
			sidecars:    []*models.Sidecar{sc1, sc2},
			genesisIDs:  []int64{42, 99},
			shouldExist: true,
		},
		{
			testName:    "both sidecars with (same) ID",
			sidecars:    []*models.Sidecar{sc1, sc2},
			sidecarIDs:  []string{"42", "42"},
			shouldExist: true,
		},
		{
			testName:    "both sidecars with (different) ID one exists",
			sidecars:    []*models.Sidecar{sc1, sc2},
			sidecarIDs:  []string{"42", "99"},
			shouldExist: true,
		},
		{
			testName:    "both sidecars with (different) ID one exists different index",
			sidecars:    []*models.Sidecar{sc1, sc2},
			sidecarIDs:  []string{"99", "42"},
			shouldExist: true,
		},
		{
			testName:    "one chainID from sidecar, other one from genesis but different",
			sidecars:    []*models.Sidecar{sc1, sc2},
			sidecarIDs:  []string{"77", ""},
			genesisIDs:  []int64{88, 99},
			shouldExist: false,
		},
		{
			testName:    "one chainID from sidecar, other one from genesis but same",
			sidecars:    []*models.Sidecar{sc1, sc2},
			sidecarIDs:  []string{"42"},
			genesisIDs:  []int64{42, 42},
			shouldExist: true,
		},
		{
			testName:    "one chainID from sidecar, other one from genesis but same different index",
			sidecars:    []*models.Sidecar{sc1, sc2},
			sidecarIDs:  []string{"", "42"},
			genesisIDs:  []int64{42, 42},
			shouldExist: true,
		},
	}

	err := os.MkdirAll(ap.GetSubnetDir(), constants.DefaultPerms755)
	require.NoError(err)

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			// set the chainIDs to the sidecars if the
			// test declares it
			for i, id := range tt.sidecarIDs {
				tt.sidecars[i].ChainID = id
			}
			// generate genesis files when needed
			// (the exists check will load the genesis if it can't find
			// the chain id in the sidecar)
			for i, id := range tt.genesisIDs {
				gen := core.Genesis{
					Config: &params.ChainConfig{
						ChainID: big.NewInt(id),
					},
					// following are required for JSON marshalling but irrelevant for the test
					Difficulty: big.NewInt(int64(42)),
					Alloc:      core.GenesisAlloc{},
				}
				genBytes, err := json.Marshal(gen)
				require.NoError(err)
				err = ap.WriteGenesisFile(tt.sidecars[i].Name, genBytes)
				require.NoError(err)
			}
			// generate the sidecars
			for _, sc := range tt.sidecars {
				scBytes, err := json.MarshalIndent(sc, "", "    ")
				require.NoError(err)
				sidecarPath := ap.GetSidecarPath(sc.Name)
				err = os.WriteFile(sidecarPath, scBytes, WriteReadReadPerms)
				require.NoError(err)
			}

			exists, err := ap.SubnetEvmChainIDExists("42")
			require.NoError(err)
			if tt.shouldExist {
				require.True(exists)
			} else {
				require.False(exists)
			}
			// cleanup files after each test...
			// remove all sidecars:
			for _, sc := range tt.sidecars {
				sidecarPath := ap.GetSidecarPath(sc.Name)
				err = os.Remove(sidecarPath)
				require.NoError(err)
			}
			// remove only genesis which actually has been created
			// or get an error on removal:
			for i := range tt.genesisIDs {
				sc := tt.sidecars[i]
				genesisPath := ap.GetGenesisPath(sc.Name)
				err = os.Remove(genesisPath)
				require.NoError(err)
			}
		})
	}
}

func Test_loadSidecar_failure_notFound(t *testing.T) {
	require := require.New(t)

	ap := newTestApp(t)

	// Assert file doesn't exist at start
	sidecarPath := ap.GetSidecarPath(subnetName1)
	_, err := os.Stat(sidecarPath)
	require.Error(err)

	_, err = ap.LoadSidecar(subnetName1)
	require.Error(err)
}

func Test_loadSidecar_failure_malformed(t *testing.T) {
	require := require.New(t)

	ap := newTestApp(t)

	// Write sidecar
	sidecarBytes := []byte("bad_sidecar")
	sidecarPath := ap.GetSidecarPath(subnetName1)
	err := os.MkdirAll(filepath.Dir(sidecarPath), constants.DefaultPerms755)
	require.NoError(err)

	err = os.WriteFile(sidecarPath, sidecarBytes, 0o600)
	require.NoError(err)

	// Check file exists
	_, err = os.Stat(sidecarPath)
	require.NoError(err)

	// Check contents
	_, err = ap.LoadSidecar(subnetName1)
	require.Error(err)

	// Cleanup file
	err = os.Remove(sidecarPath)
	require.NoError(err)
}

func Test_genesisExists(t *testing.T) {
	require := require.New(t)

	ap := newTestApp(t)

	// Assert file doesn't exist at start
	result := ap.GenesisExists(subnetName1)
	require.False(result)

	// Create genesis
	genesisPath := ap.GetGenesisPath(subnetName1)
	genesisBytes := []byte("genesis")
	err := os.MkdirAll(filepath.Dir(genesisPath), constants.DefaultPerms755)
	require.NoError(err)
	err = os.WriteFile(genesisPath, genesisBytes, 0o600)
	require.NoError(err)

	// Verify genesis exists
	result = ap.GenesisExists(subnetName1)
	require.True(result)

	// Clean up created genesis
	err = os.Remove(genesisPath)
	require.NoError(err)
}

func newTestApp(t *testing.T) *Avalanche {
	tempDir := t.TempDir()
	return &Avalanche{
		baseDir: tempDir,
		Log:     logging.NoLog{},
	}
}
