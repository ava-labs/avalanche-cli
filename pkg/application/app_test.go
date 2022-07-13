// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package application

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/params"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

const (
	subnetName1 = "TEST_subnet"
	subnetName2 = "TEST_copied_subnet"
)

func Test_writeGenesisFile_success(t *testing.T) {
	assert := assert.New(t)
	genesisBytes := []byte("genesis")
	genesisFile := subnetName1 + constants.GenesisSuffix

	ap := newTestApp(t)
	// Write genesis
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	assert.NoError(err)

	// Check file exists
	createdPath := filepath.Join(ap.GetBaseDir(), genesisFile)
	_, err = os.Stat(createdPath)
	assert.NoError(err)

	// Cleanup file
	err = os.Remove(createdPath)
	assert.NoError(err)
}

func Test_copyGenesisFile_success(t *testing.T) {
	assert := assert.New(t)
	genesisBytes := []byte("genesis")

	genesisFile1 := subnetName1 + constants.GenesisSuffix
	genesisFile2 := subnetName2 + constants.GenesisSuffix

	ap := newTestApp(t)
	// Create original genesis
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	assert.NoError(err)

	// Copy genesis
	createdGenesis := filepath.Join(ap.GetBaseDir(), genesisFile1)
	err = ap.CopyGenesisFile(createdGenesis, subnetName2)
	assert.NoError(err)

	// Check copied file exists
	copiedGenesis := filepath.Join(ap.GetBaseDir(), genesisFile2)
	_, err = os.Stat(copiedGenesis)
	assert.NoError(err)

	// Cleanup files
	err = os.Remove(createdGenesis)
	assert.NoError(err)
	err = os.Remove(copiedGenesis)
	assert.NoError(err)
}

func Test_copyGenesisFile_failure(t *testing.T) {
	assert := assert.New(t)
	// copy genesis that doesn't exist
	genesisFile1 := subnetName1 + constants.GenesisSuffix
	genesisFile2 := subnetName2 + constants.GenesisSuffix

	ap := newTestApp(t)
	// Copy genesis
	createdGenesis := filepath.Join(ap.GetBaseDir(), genesisFile1)
	err := ap.CopyGenesisFile(createdGenesis, subnetName2)
	assert.Error(err)

	// Check no copied file exists
	copiedGenesis := filepath.Join(ap.GetBaseDir(), genesisFile2)
	_, err = os.Stat(copiedGenesis)
	assert.Error(err)
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
			assert := assert.New(t)
			sidecarFile := tt.subnetName + constants.SidecarSuffix
			const vm = models.SubnetEvm

			sc := &models.Sidecar{
				Name:      tt.subnetName,
				VM:        vm,
				TokenName: tt.tokenName,
				ChainID:   tt.chainID,
			}

			// Write sidecar
			err := ap.CreateSidecar(sc)
			assert.NoError(err)

			// Check file exists
			createdPath := filepath.Join(ap.GetBaseDir(), sidecarFile)
			_, err = os.Stat(createdPath)
			assert.NoError(err)

			control, err := ap.LoadSidecar(tt.subnetName)
			assert.NoError(err)
			assert.Equal(*sc, control)

			assert.Equal(sc.TokenName, tt.expectedTokenName)

			// Cleanup file
			err = os.Remove(createdPath)
			assert.NoError(err)
		})
	}
}

func Test_loadSidecar_success(t *testing.T) {
	assert := assert.New(t)
	sidecarFile := subnetName1 + constants.SidecarSuffix
	const vm = models.SubnetEvm

	ap := newTestApp(t)

	// Write sidecar
	sidecarBytes := []byte("{  \"Name\": \"TEST_subnet\",\n  \"VM\": \"SubnetEVM\",\n  \"Subnet\": \"TEST_subnet\"\n  }")
	sidecarPath := filepath.Join(ap.GetBaseDir(), sidecarFile)
	err := os.WriteFile(sidecarPath, sidecarBytes, 0o600)
	assert.NoError(err)

	// Check file exists
	_, err = os.Stat(sidecarPath)
	assert.NoError(err)

	// Check contents
	expectedSc := models.Sidecar{
		Name:      subnetName1,
		VM:        vm,
		Subnet:    subnetName1,
		TokenName: constants.DefaultTokenName,
	}

	sc, err := ap.LoadSidecar(subnetName1)
	assert.NoError(err)
	assert.Equal(sc, expectedSc)

	// Cleanup file
	err = os.Remove(sidecarPath)
	assert.NoError(err)
}

func TestChainIDExists(t *testing.T) {
	assert := assert.New(t)

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
				assert.NoError(err)
				err = ap.WriteGenesisFile(tt.sidecars[i].Name, genBytes)
				assert.NoError(err)
			}
			// generate the sidecars
			for _, sc := range tt.sidecars {
				scBytes, err := json.MarshalIndent(sc, "", "    ")
				assert.NoError(err)
				sidecarPath := ap.GetSidecarPath(sc.Name)
				err = os.WriteFile(sidecarPath, scBytes, WriteReadReadPerms)
				assert.NoError(err)
			}

			exists, err := ap.ChainIDExists("42")
			assert.NoError(err)
			if tt.shouldExist {
				assert.True(exists)
			} else {
				assert.False(exists)
			}
			// cleanup files after each test...
			// remove all sidecars:
			for _, sc := range tt.sidecars {
				sidecarPath := ap.GetSidecarPath(sc.Name)
				err = os.Remove(sidecarPath)
				assert.NoError(err)
			}
			// remove only genesis which actually has been created
			// or get an error on removal:
			for i := range tt.genesisIDs {
				sc := tt.sidecars[i]
				genesisPath := ap.GetGenesisPath(sc.Name)
				err = os.Remove(genesisPath)
				assert.NoError(err)
			}
		})
	}
}

func Test_failure_duplicateChainID(t *testing.T) {
	assert := assert.New(t)
	sc1 := &models.Sidecar{
		Name:      "sc1",
		VM:        models.SubnetEvm,
		TokenName: "TEST",
		ChainID:   "42",
	}

	sc2 := &models.Sidecar{
		Name:      "sc2",
		VM:        models.SubnetEvm,
		TokenName: "TEST",
		ChainID:   "42",
	}

	ap := newTestApp(t)

	err := ap.CreateSidecar(sc1)
	assert.NoError(err)

	err = ap.CreateSidecar(sc2)
	assert.ErrorIs(err, errChainIDExists)
}

func Test_loadSidecar_failure_notFound(t *testing.T) {
	assert := assert.New(t)
	sidecarFile := subnetName1 + constants.SidecarSuffix

	ap := newTestApp(t)

	// Assert file doesn't exist at start
	sidecarPath := filepath.Join(ap.GetBaseDir(), sidecarFile)
	_, err := os.Stat(sidecarPath)
	assert.Error(err)

	_, err = ap.LoadSidecar(subnetName1)
	assert.Error(err)
}

func Test_loadSidecar_failure_malformed(t *testing.T) {
	assert := assert.New(t)
	sidecarFile := subnetName1 + constants.SidecarSuffix

	ap := newTestApp(t)

	// Write sidecar
	sidecarBytes := []byte("bad_sidecar")
	sidecarPath := filepath.Join(ap.GetBaseDir(), sidecarFile)
	err := os.WriteFile(sidecarPath, sidecarBytes, 0o600)
	assert.NoError(err)

	// Check file exists
	_, err = os.Stat(sidecarPath)
	assert.NoError(err)

	// Check contents
	_, err = ap.LoadSidecar(subnetName1)
	assert.Error(err)

	// Cleanup file
	err = os.Remove(sidecarPath)
	assert.NoError(err)
}

func Test_genesisExists(t *testing.T) {
	assert := assert.New(t)
	genesisFile := subnetName1 + constants.GenesisSuffix

	ap := newTestApp(t)

	// Assert file doesn't exist at start
	result := ap.GenesisExists(subnetName1)
	assert.False(result)

	// Create genesis
	genesisPath := filepath.Join(ap.GetBaseDir(), genesisFile)
	genesisBytes := []byte("genesis")
	err := os.WriteFile(genesisPath, genesisBytes, 0o600)
	assert.NoError(err)

	// Verify genesis exists
	result = ap.GenesisExists(subnetName1)
	assert.True(result)

	// Clean up created genesis
	err = os.Remove(genesisPath)
	assert.NoError(err)
}

func Test_LoadNodeConfig(t *testing.T) {
	assert := assert.New(t)
	ap := newTestApp(t)

	err := useViper("node-config-test")
	assert.NoError(err)

	config, err := ap.LoadNodeConfig()
	assert.NoError(err)
	fmt.Println("Config:", config)
	testVal := viper.GetString("var")
	fmt.Println("Test val", testVal)
	assert.Equal("val", testVal)
}

func Test_LoadNodeConfig_EmptyConfig(t *testing.T) {
	assert := assert.New(t)
	ap := newTestApp(t)

	err := useViper("empty-config")
	assert.NoError(err)

	config, err := ap.LoadNodeConfig()
	assert.NoError(err)
	assert.Empty(config)
}

func Test_LoadNodeConfig_NoConfig(t *testing.T) {
	assert := assert.New(t)
	ap := newTestApp(t)

	err := useViper("")
	// we want to make sure this errors and no config file is read
	assert.Error(err)

	config, err := ap.LoadNodeConfig()
	assert.NoError(err)
	assert.Empty(config)
}

func newTestApp(t *testing.T) *Avalanche {
	tempDir := t.TempDir()
	return &Avalanche{
		baseDir: tempDir,
		Log:     logging.NoLog{},
	}
}

func useViper(configName string) error {
	viper.Reset()
	viper.SetConfigName(configName)
	viper.SetConfigType("json")
	viper.AddConfigPath("./../../tests/assets/")

	return viper.ReadInConfig()
}
