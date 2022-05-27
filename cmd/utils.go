// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/subnet-evm/core"
)

const WRITE_READ_READ_PERMS = 0644

func writeGenesisFile(subnetName string, genesisBytes []byte) error {
	genesisPath := filepath.Join(baseDir, subnetName+genesis_suffix)
	return os.WriteFile(genesisPath, genesisBytes, WRITE_READ_READ_PERMS)
}

func genesisExists(subnetName string) bool {
	genesisPath := filepath.Join(baseDir, subnetName+genesis_suffix)
	_, err := os.Stat(genesisPath)
	return err == nil
}

func copyGenesisFile(inputFilename string, subnetName string) error {
	genesisBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	genesisPath := filepath.Join(baseDir, subnetName+genesis_suffix)
	return os.WriteFile(genesisPath, genesisBytes, WRITE_READ_READ_PERMS)
}

func loadEvmGenesis(subnetName string) (core.Genesis, error) {
	genesisPath := filepath.Join(baseDir, subnetName+genesis_suffix)
	jsonBytes, err := os.ReadFile(genesisPath)
	if err != nil {
		return core.Genesis{}, err
	}

	var gen core.Genesis
	err = json.Unmarshal(jsonBytes, &gen)
	return gen, err
}

func createSidecar(subnetName string, vm models.VmType) error {
	sc := models.Sidecar{
		Name:   subnetName,
		Vm:     vm,
		Subnet: subnetName,
	}

	scBytes, err := json.MarshalIndent(sc, "", "    ")
	if err != nil {
		return nil
	}

	sidecarPath := filepath.Join(baseDir, subnetName+sidecar_suffix)
	return os.WriteFile(sidecarPath, scBytes, WRITE_READ_READ_PERMS)
}

func loadSidecar(subnetName string) (models.Sidecar, error) {
	sidecarPath := filepath.Join(baseDir, subnetName+sidecar_suffix)
	jsonBytes, err := os.ReadFile(sidecarPath)
	if err != nil {
		return models.Sidecar{}, err
	}

	var sc models.Sidecar
	err = json.Unmarshal(jsonBytes, &sc)
	return sc, err
}
