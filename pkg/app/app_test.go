// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
)

func Test_writeGenesisFile_success(t *testing.T) {
	genesisBytes := []byte("genesis")
	subnetName := "TEST_subnet"
	genesisFile := subnetName + constants.Genesis_suffix

	ap := newTestApp(t)
	// Write genesis
	err := ap.WriteGenesisFile(subnetName, genesisBytes)
	assert.NoError(t, err)

	// Check file exists
	createdPath := filepath.Join(ap.GetBaseDir(), genesisFile)
	_, err = os.Stat(createdPath)
	assert.NoError(t, err)

	// Cleanup file
	err = os.Remove(createdPath)
	assert.NoError(t, err)
}

func Test_copyGenesisFile_success(t *testing.T) {
	genesisBytes := []byte("genesis")
	subnetName1 := "TEST_subnet"
	subnetName2 := "TEST_copied_subnet"
	genesisFile1 := subnetName1 + constants.Genesis_suffix
	genesisFile2 := subnetName2 + constants.Genesis_suffix

	ap := newTestApp(t)
	// Create original genesis
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	assert.NoError(t, err)

	// Copy genesis
	createdGenesis := filepath.Join(ap.GetBaseDir(), genesisFile1)
	err = ap.CopyGenesisFile(createdGenesis, subnetName2)
	assert.NoError(t, err)

	// Check copied file exists
	copiedGenesis := filepath.Join(ap.GetBaseDir(), genesisFile2)
	_, err = os.Stat(copiedGenesis)
	assert.NoError(t, err)

	// Cleanup files
	err = os.Remove(createdGenesis)
	assert.NoError(t, err)
	err = os.Remove(copiedGenesis)
	assert.NoError(t, err)
}

func Test_copyGenesisFile_failure(t *testing.T) {
	// copy genesis that doesn't exist
	subnetName1 := "TEST_subnet"
	subnetName2 := "TEST_copied_subnet"
	genesisFile1 := subnetName1 + constants.Genesis_suffix
	genesisFile2 := subnetName2 + constants.Genesis_suffix

	ap := newTestApp(t)
	// Copy genesis
	createdGenesis := filepath.Join(ap.GetBaseDir(), genesisFile1)
	err := ap.CopyGenesisFile(createdGenesis, subnetName2)
	assert.Error(t, err)

	// Check no copied file exists
	copiedGenesis := filepath.Join(ap.GetBaseDir(), genesisFile2)
	_, err = os.Stat(copiedGenesis)
	assert.Error(t, err)
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
			subnetName:        "TEST_subnet",
			tokenName:         "TOKEN",
			expectedTokenName: "TOKEN",
			chainID:           "999",
		},
		{
			name:              "no token name",
			subnetName:        "TEST_subnet",
			tokenName:         "",
			expectedTokenName: "TEST",
			chainID:           "888",
		},
	}

	ap := newTestApp(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			sidecarFile := tt.subnetName + constants.Sidecar_suffix
			const vm = models.SubnetEvm

			sc := &models.Sidecar{
				Name:      tt.subnetName,
				Vm:        vm,
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
	subnetName := "TEST_subnet"
	sidecarFile := subnetName + constants.Sidecar_suffix
	const vm = models.SubnetEvm

	ap := newTestApp(t)

	// Write sidecar
	sidecarBytes := []byte("{  \"Name\": \"TEST_subnet\",\n  \"Vm\": \"SubnetEVM\",\n  \"Subnet\": \"TEST_subnet\"\n  }")
	sidecarPath := filepath.Join(ap.GetBaseDir(), sidecarFile)
	err := os.WriteFile(sidecarPath, sidecarBytes, 0o644)
	assert.NoError(t, err)

	// Check file exists
	_, err = os.Stat(sidecarPath)
	assert.NoError(t, err)

	// Check contents
	expectedSc := models.Sidecar{
		Name:   subnetName,
		Vm:     vm,
		Subnet: subnetName,
	}

	sc, err := ap.LoadSidecar(subnetName)
	assert.NoError(t, err)
	assert.Equal(t, sc, expectedSc)

	// Cleanup file
	err = os.Remove(sidecarPath)
	assert.NoError(t, err)
}

func Test_failure_duplicateChainID(t *testing.T) {
	assert := assert.New(t)
	sc1 := &models.Sidecar{
		Name:      "sc1",
		Vm:        models.SubnetEvm,
		TokenName: "TEST",
		ChainID:   "42",
	}

	sc2 := &models.Sidecar{
		Name:      "sc2",
		Vm:        models.SubnetEvm,
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
	subnetName := "TEST_subnet"
	sidecarFile := subnetName + constants.Sidecar_suffix

	ap := newTestApp(t)

	// Assert file doesn't exist at start
	sidecarPath := filepath.Join(ap.GetBaseDir(), sidecarFile)
	_, err := os.Stat(sidecarPath)
	assert.Error(t, err)

	_, err = ap.LoadSidecar(subnetName)
	assert.Error(t, err)
}

func Test_loadSidecar_failure_malformed(t *testing.T) {
	subnetName := "TEST_subnet"
	sidecarFile := subnetName + constants.Sidecar_suffix

	ap := newTestApp(t)

	// Write sidecar
	sidecarBytes := []byte("bad_sidecar")
	sidecarPath := filepath.Join(ap.GetBaseDir(), sidecarFile)
	err := os.WriteFile(sidecarPath, sidecarBytes, 0o644)
	assert.NoError(t, err)

	// Check file exists
	_, err = os.Stat(sidecarPath)
	assert.NoError(t, err)

	// Check contents
	_, err = ap.LoadSidecar(subnetName)
	assert.Error(t, err)

	// Cleanup file
	err = os.Remove(sidecarPath)
	assert.NoError(t, err)
}

func Test_genesisExists(t *testing.T) {
	subnetName := "TEST_subnet"
	genesisFile := subnetName + constants.Genesis_suffix

	ap := newTestApp(t)

	// Assert file doesn't exist at start
	result := ap.GenesisExists(subnetName)
	assert.False(t, result)

	// Create genesis
	genesisPath := filepath.Join(ap.GetBaseDir(), genesisFile)
	genesisBytes := []byte("genesis")
	err := os.WriteFile(genesisPath, genesisBytes, 0o644)
	assert.NoError(t, err)

	// Verify genesis exists
	result = ap.GenesisExists(subnetName)
	assert.True(t, result)

	// Clean up created genesis
	err = os.Remove(genesisPath)
	assert.NoError(t, err)
}

func newTestApp(t *testing.T) *Avalanche {
	tempDir := t.TempDir()
	return &Avalanche{
		baseDir: tempDir,
		Log:     logging.NoLog{},
	}
}
