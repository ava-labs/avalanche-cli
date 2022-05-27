// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/stretchr/testify/assert"
)

func Test_writeGenesisFile_success(t *testing.T) {
	genesisBytes := []byte("genesis")
	subnetName := "TEST_subnet"
	genesisFile := subnetName + genesis_suffix

	// Write genesis
	err := writeGenesisFile(subnetName, genesisBytes)
	assert.NoError(t, err)

	// Check file exists
	createdPath := filepath.Join(baseDir, genesisFile)
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
	genesisFile1 := subnetName1 + genesis_suffix
	genesisFile2 := subnetName2 + genesis_suffix

	// Create original genesis
	err := writeGenesisFile(subnetName1, genesisBytes)
	assert.NoError(t, err)

	// Copy genesis
	createdGenesis := filepath.Join(baseDir, genesisFile1)
	err = copyGenesisFile(createdGenesis, subnetName2)
	assert.NoError(t, err)

	// Check copied file exists
	copiedGenesis := filepath.Join(baseDir, genesisFile2)
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
	genesisFile1 := subnetName1 + genesis_suffix
	genesisFile2 := subnetName2 + genesis_suffix

	// Copy genesis
	createdGenesis := filepath.Join(baseDir, genesisFile1)
	err := copyGenesisFile(createdGenesis, subnetName2)
	assert.Error(t, err)

	// Check no copied file exists
	copiedGenesis := filepath.Join(baseDir, genesisFile2)
	_, err = os.Stat(copiedGenesis)
	assert.Error(t, err)
}

func Test_createSidecar_success(t *testing.T) {
	subnetName := "TEST_subnet"
	sidecarFile := subnetName + sidecar_suffix
	const vm = models.SubnetEvm

	// Write sidecar
	err := createSidecar(subnetName, vm)
	assert.NoError(t, err)

	// Check file exists
	createdPath := filepath.Join(baseDir, sidecarFile)
	_, err = os.Stat(createdPath)
	assert.NoError(t, err)

	// Check contents
	expectedSc := models.Sidecar{
		Name:   subnetName,
		Vm:     vm,
		Subnet: subnetName,
	}

	sc, err := loadSidecar(subnetName)
	assert.NoError(t, err)
	assert.Equal(t, sc, expectedSc)

	// Cleanup file
	err = os.Remove(createdPath)
	assert.NoError(t, err)
}

func Test_loadSidecar_success(t *testing.T) {
	subnetName := "TEST_subnet"
	sidecarFile := subnetName + sidecar_suffix
	const vm = models.SubnetEvm

	// Write sidecar
	sidecarBytes := []byte("{  \"Name\": \"TEST_subnet\",\n  \"Vm\": \"SubnetEVM\",\n  \"Subnet\": \"TEST_subnet\"\n  }")
	sidecarPath := filepath.Join(baseDir, sidecarFile)
	err := os.WriteFile(sidecarPath, sidecarBytes, 0644)
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

	sc, err := loadSidecar(subnetName)
	assert.NoError(t, err)
	assert.Equal(t, sc, expectedSc)

	// Cleanup file
	err = os.Remove(sidecarPath)
	assert.NoError(t, err)
}

func Test_loadSidecar_failure_notFound(t *testing.T) {
	subnetName := "TEST_subnet"
	sidecarFile := subnetName + sidecar_suffix

	// Assert file doesn't exist at start
	sidecarPath := filepath.Join(baseDir, sidecarFile)
	_, err := os.Stat(sidecarPath)
	assert.Error(t, err)

	_, err = loadSidecar(subnetName)
	assert.Error(t, err)
}

func Test_loadSidecar_failure_malformed(t *testing.T) {
	subnetName := "TEST_subnet"
	sidecarFile := subnetName + sidecar_suffix

	// Write sidecar
	sidecarBytes := []byte("bad_sidecar")
	sidecarPath := filepath.Join(baseDir, sidecarFile)
	err := os.WriteFile(sidecarPath, sidecarBytes, 0644)
	assert.NoError(t, err)

	// Check file exists
	_, err = os.Stat(sidecarPath)
	assert.NoError(t, err)

	// Check contents
	_, err = loadSidecar(subnetName)
	assert.Error(t, err)

	// Cleanup file
	err = os.Remove(sidecarPath)
	assert.NoError(t, err)
}

func Test_genesisExists(t *testing.T) {
	subnetName := "TEST_subnet"
	genesisFile := subnetName + genesis_suffix

	// Assert file doesn't exist at start
	result := genesisExists(subnetName)
	assert.False(t, result)

	// Create genesis
	genesisPath := filepath.Join(baseDir, genesisFile)
	genesisBytes := []byte("genesis")
	err := os.WriteFile(genesisPath, genesisBytes, 0644)
	assert.NoError(t, err)

	// Verify genesis exists
	result = genesisExists(subnetName)
	assert.True(t, result)

	// Clean up created genesis
	err = os.Remove(genesisPath)
	assert.NoError(t, err)
}
