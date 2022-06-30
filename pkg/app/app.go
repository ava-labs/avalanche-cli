// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
)

const (
	WriteReadReadPerms = 0o644
)

var errChainIDExists = errors.New("the provided chain ID already exists! Try another one")

type Avalanche struct {
	Log     logging.Logger
	baseDir string
	runFile string
}

func New(baseDir string, log logging.Logger) *Avalanche {
	return &Avalanche{
		baseDir: baseDir,
		Log:     log,
	}
}

func (app *Avalanche) GetRunFile() string {
	return filepath.Join(app.GetRunDir(), constants.ServerRunFile)
}

func (app *Avalanche) GetBaseDir() string {
	return app.baseDir
}

func (app *Avalanche) GetRunDir() string {
	return filepath.Join(app.baseDir, constants.RunDir)
}

func (app *Avalanche) GetGenesisPath(subnetName string) string {
	return filepath.Join(app.baseDir, subnetName+constants.GenesisSuffix)
}

func (app *Avalanche) GetSidecarPath(subnetName string) string {
	return filepath.Join(app.baseDir, subnetName+constants.SidecarSuffix)
}

func (app *Avalanche) WriteGenesisFile(subnetName string, genesisBytes []byte) error {
	genesisPath := app.GetGenesisPath(subnetName)
	return os.WriteFile(genesisPath, genesisBytes, WriteReadReadPerms)
}

func (app *Avalanche) GenesisExists(subnetName string) bool {
	genesisPath := app.GetGenesisPath(subnetName)
	_, err := os.Stat(genesisPath)
	return err == nil
}

func (app *Avalanche) CopyGenesisFile(inputFilename string, subnetName string) error {
	genesisBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	genesisPath := app.GetGenesisPath(subnetName)
	return os.WriteFile(genesisPath, genesisBytes, WriteReadReadPerms)
}

func (app *Avalanche) LoadEvmGenesis(subnetName string) (core.Genesis, error) {
	genesisPath := app.GetGenesisPath(subnetName)
	jsonBytes, err := os.ReadFile(genesisPath)
	if err != nil {
		return core.Genesis{}, err
	}

	var gen core.Genesis
	err = json.Unmarshal(jsonBytes, &gen)
	return gen, err
}

func (app *Avalanche) CreateSidecar(sc *models.Sidecar) error {
	if sc.TokenName == "" {
		sc.TokenName = constants.DefaultTokenName
	}
	// We should have caught this during the actual prompting,
	// but better safe than sorry
	exists, err := app.ChainIDExists(sc.ChainID)
	if err != nil {
		return err
	}
	if exists {
		return errChainIDExists
	}
	// only apply the version on a write
	sc.Version = constants.SidecarVersion
	scBytes, err := json.MarshalIndent(sc, "", "    ")
	if err != nil {
		return nil
	}

	sidecarPath := app.GetSidecarPath(sc.Name)
	return os.WriteFile(sidecarPath, scBytes, WriteReadReadPerms)
}

func (app *Avalanche) LoadSidecar(subnetName string) (models.Sidecar, error) {
	sidecarPath := app.GetSidecarPath(subnetName)
	jsonBytes, err := os.ReadFile(sidecarPath)
	if err != nil {
		return models.Sidecar{}, err
	}

	var sc models.Sidecar
	err = json.Unmarshal(jsonBytes, &sc)

	if sc.TokenName == "" {
		sc.TokenName = constants.DefaultTokenName
	}

	return sc, err
}

func (app *Avalanche) GetTokenName(subnetName string) string {
	sidecar, err := app.LoadSidecar(subnetName)
	if err != nil {
		return constants.DefaultTokenName
	}
	return sidecar.TokenName
}

func (app *Avalanche) listSideCarNames() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(app.baseDir, "*"+constants.SidecarSuffix))
	if err != nil {
		return nil, err
	}
	names := make([]string, len(matches))
	for i, m := range matches {
		base := filepath.Base(m)
		name := base[:len(base)-len(constants.SidecarSuffix)]
		names[i] = name
	}
	return names, nil
}

func (app *Avalanche) ChainIDExists(chainID string) (bool, error) {
	sidecars, err := app.listSideCarNames()
	if err != nil {
		return false, err
	}
	for _, car := range sidecars {
		sc, err := app.LoadSidecar(car)
		if err != nil {
			return false, err
		}
		existingChainID := sc.ChainID
		// sidecar doesn't contain chain ID yet
		// try loading it from genesis
		if sc.ChainID == "" {
			gen, err := app.LoadEvmGenesis(car)
			if err != nil {
				return false, err
			}
			existingChainID = gen.Config.ChainID.String()
		}
		if existingChainID == chainID {
			return true, nil
		}
	}

	return false, nil
}
