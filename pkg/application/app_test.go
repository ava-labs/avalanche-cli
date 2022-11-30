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
	"github.com/stretchr/testify/assert"
)

const (
	subnetName1 = "TEST_subnet"
	subnetName2 = "TEST_copied_subnet"
)

func TestUpdateSideCar(t *testing.T) {
	assert := assert.New(t)
	sc := &models.Sidecar{
		Name:      "TEST",
		VM:        models.SubnetEvm,
		TokenName: "TEST",
		ChainID:   "42",
	}

	ap := newTestApp(t)

	err := ap.CreateSidecar(sc)
	assert.NoError(err)
	control, err := ap.LoadSidecar(sc.Name)
	assert.NoError(err)
	assert.Equal(*sc, control)
	sc.Networks = make(map[string]models.NetworkData)
	sc.Networks["local"] = models.NetworkData{
		BlockchainID: ids.GenerateTestID(),
		SubnetID:     ids.GenerateTestID(),
	}

	err = ap.UpdateSidecar(sc)
	assert.NoError(err)
	control, err = ap.LoadSidecar(sc.Name)
	assert.NoError(err)
	assert.Equal(*sc, control)
}

func Test_writeGenesisFile_success(t *testing.T) {
	assert := assert.New(t)
	genesisBytes := []byte("genesis")
	genesisFile := constants.GenesisFileName

	ap := newTestApp(t)
	// Write genesis
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	assert.NoError(err)

	// Check file exists
	createdPath := filepath.Join(ap.GetSubnetDir(), subnetName1, genesisFile)
	_, err = os.Stat(createdPath)
	assert.NoError(err)

	// Cleanup file
	err = os.Remove(createdPath)
	assert.NoError(err)
}

func Test_copyGenesisFile_success(t *testing.T) {
	assert := assert.New(t)
	genesisBytes := []byte("genesis")

	ap := newTestApp(t)
	// Create original genesis
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	assert.NoError(err)

	// Copy genesis
	createdGenesis := ap.GetGenesisPath(subnetName1)
	err = ap.CopyGenesisFile(createdGenesis, subnetName2)
	assert.NoError(err)

	// Check copied file exists
	copiedGenesis := ap.GetGenesisPath(subnetName2)
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

	ap := newTestApp(t)
	// Copy genesis
	createdGenesis := ap.GetGenesisPath(subnetName1)
	err := ap.CopyGenesisFile(createdGenesis, subnetName2)
	assert.Error(err)

	// Check no copied file exists
	copiedGenesis := ap.GetGenesisPath(subnetName2)
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
			createdPath := ap.GetSidecarPath(tt.subnetName)
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
	const vm = models.SubnetEvm

	ap := newTestApp(t)

	// Write sidecar
	sidecarBytes := []byte("{  \"Name\": \"TEST_subnet\",\n  \"VM\": \"SubnetEVM\",\n  \"Subnet\": \"TEST_subnet\"\n  }")
	sidecarPath := ap.GetSidecarPath(subnetName1)
	err := os.MkdirAll(filepath.Dir(sidecarPath), constants.DefaultPerms755)
	assert.NoError(err)

	err = os.WriteFile(sidecarPath, sidecarBytes, 0o600)
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

func Test_loadSidecar_failure_notFound(t *testing.T) {
	assert := assert.New(t)

	ap := newTestApp(t)

	// Assert file doesn't exist at start
	sidecarPath := ap.GetSidecarPath(subnetName1)
	_, err := os.Stat(sidecarPath)
	assert.Error(err)

	_, err = ap.LoadSidecar(subnetName1)
	assert.Error(err)
}

func Test_loadSidecar_failure_malformed(t *testing.T) {
	assert := assert.New(t)

	ap := newTestApp(t)

	// Write sidecar
	sidecarBytes := []byte("bad_sidecar")
	sidecarPath := ap.GetSidecarPath(subnetName1)
	err := os.MkdirAll(filepath.Dir(sidecarPath), constants.DefaultPerms755)
	assert.NoError(err)

	err = os.WriteFile(sidecarPath, sidecarBytes, 0o600)
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

	ap := newTestApp(t)

	// Assert file doesn't exist at start
	result := ap.GenesisExists(subnetName1)
	assert.False(result)

	// Create genesis
	genesisPath := ap.GetGenesisPath(subnetName1)
	genesisBytes := []byte("genesis")
	err := os.MkdirAll(filepath.Dir(genesisPath), constants.DefaultPerms755)
	assert.NoError(err)
	err = os.WriteFile(genesisPath, genesisBytes, 0o600)
	assert.NoError(err)

	// Verify genesis exists
	result = ap.GenesisExists(subnetName1)
	assert.True(result)

	// Clean up created genesis
	err = os.Remove(genesisPath)
	assert.NoError(err)
}

func newTestApp(t *testing.T) *Avalanche {
	tempDir := t.TempDir()
	return &Avalanche{
		baseDir: tempDir,
		Log:     logging.NoLog{},
	}
}
