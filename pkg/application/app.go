// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/apm/apm"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
)

const (
	WriteReadReadPerms = 0o644
)

type Avalanche struct {
	Log        logging.Logger
	baseDir    string
	Conf       *config.Config
	Prompt     prompts.Prompter
	Apm        *apm.APM
	ApmDir     string
	Downloader Downloader
}

func New() *Avalanche {
	return &Avalanche{}
}

func (app *Avalanche) Setup(baseDir string, log logging.Logger, conf *config.Config, prompt prompts.Prompter, downloader Downloader) {
	app.baseDir = baseDir
	app.Log = log
	app.Conf = conf
	app.Prompt = prompt
	app.Downloader = downloader
}

func (app *Avalanche) GetRunFile() string {
	return filepath.Join(app.GetRunDir(), constants.ServerRunFile)
}

func (app *Avalanche) GetSnapshotsDir() string {
	return filepath.Join(app.baseDir, constants.SnapshotsDirName)
}

func (app *Avalanche) GetBaseDir() string {
	return app.baseDir
}

func (app *Avalanche) GetSubnetDir() string {
	return filepath.Join(app.baseDir, constants.SubnetDir)
}

func (app *Avalanche) GetReposDir() string {
	return filepath.Join(app.baseDir, constants.ReposDir)
}

func (app *Avalanche) GetRunDir() string {
	return filepath.Join(app.baseDir, constants.RunDir)
}

func (app *Avalanche) GetCustomVMDir() string {
	return filepath.Join(app.baseDir, constants.CustomVMDir)
}

func (app *Avalanche) GetAvalanchegoBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.AvalancheGoInstallDir)
}

func (app *Avalanche) GetSubnetEVMBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.SubnetEVMInstallDir)
}

func (app *Avalanche) GetSpacesVMBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.SpacesVMInstallDir)
}

func (app *Avalanche) GetUpgradeBytesFilepath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.UpgradeBytesFileName)
}

func (app *Avalanche) GetCustomVMPath(subnetName string) string {
	return filepath.Join(app.GetCustomVMDir(), subnetName)
}

func (app *Avalanche) GetAPMVMPath(vmid string) string {
	return filepath.Join(app.GetAPMPluginDir(), vmid)
}

func (app *Avalanche) GetGenesisPath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.GenesisFileName)
}

func (app *Avalanche) GetSidecarPath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.SidecarFileName)
}

func (app *Avalanche) GetKeyDir() string {
	return filepath.Join(app.baseDir, constants.KeyDir)
}

func (*Avalanche) GetTmpPluginDir() string {
	return os.TempDir()
}

func (app *Avalanche) GetAPMBaseDir() string {
	return filepath.Join(app.baseDir, "apm")
}

func (app *Avalanche) GetAPMLog() string {
	return filepath.Join(app.baseDir, constants.LogDir, constants.APMLogName)
}

func (app *Avalanche) GetAPMPluginDir() string {
	return filepath.Join(app.baseDir, constants.APMPluginDir)
}

func (app *Avalanche) GetKeyPath(keyName string) string {
	return filepath.Join(app.baseDir, constants.KeyDir, keyName+constants.KeySuffix)
}

func (app *Avalanche) GetUpgradeBytesFilePath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.UpgradeBytesFileName)
}

func (app *Avalanche) GetDownloader() Downloader {
	return app.Downloader
}

func (*Avalanche) GetAvalanchegoCompatibilityURL() string {
	return constants.AvalancheGoCompatibilityURL
}

func (app *Avalanche) WriteGenesisFile(subnetName string, genesisBytes []byte) error {
	genesisPath := app.GetGenesisPath(subnetName)
	if err := os.MkdirAll(filepath.Dir(genesisPath), constants.DefaultPerms755); err != nil {
		return err
	}

	return os.WriteFile(genesisPath, genesisBytes, WriteReadReadPerms)
}

func (app *Avalanche) GenesisExists(subnetName string) bool {
	genesisPath := app.GetGenesisPath(subnetName)
	_, err := os.Stat(genesisPath)
	return err == nil
}

func (app *Avalanche) SidecarExists(subnetName string) bool {
	sidecarPath := app.GetSidecarPath(subnetName)
	_, err := os.Stat(sidecarPath)
	return err == nil
}

func (app *Avalanche) SubnetConfigExists(subnetName string) bool {
	// There's always a sidecar, but imported subnets don't have a genesis right now
	return app.SidecarExists(subnetName)
}

func (app *Avalanche) KeyExists(keyName string) bool {
	keyPath := app.GetKeyPath(keyName)
	_, err := os.Stat(keyPath)
	return err == nil
}

func (app *Avalanche) CopyGenesisFile(inputFilename string, subnetName string) error {
	genesisBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	genesisPath := app.GetGenesisPath(subnetName)
	if err := os.MkdirAll(filepath.Dir(genesisPath), constants.DefaultPerms755); err != nil {
		return err
	}

	return os.WriteFile(genesisPath, genesisBytes, WriteReadReadPerms)
}

func (app *Avalanche) CopyVMBinary(inputFilename string, subnetName string) error {
	vmBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	vmPath := app.GetCustomVMPath(subnetName)
	return os.WriteFile(vmPath, vmBytes, WriteReadReadPerms)
}

func (app *Avalanche) CopyKeyFile(inputFilename string, keyName string) error {
	keyBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	keyPath := app.GetKeyPath(keyName)
	return os.WriteFile(keyPath, keyBytes, WriteReadReadPerms)
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

func (app *Avalanche) LoadRawGenesis(subnetName string) ([]byte, error) {
	genesisPath := app.GetGenesisPath(subnetName)
	genesisBytes, err := os.ReadFile(genesisPath)
	if err != nil {
		return nil, err
	}

	return genesisBytes, err
}

func (app *Avalanche) CreateSidecar(sc *models.Sidecar) error {
	if sc.TokenName == "" {
		sc.TokenName = constants.DefaultTokenName
	}

	sidecarPath := app.GetSidecarPath(sc.Name)
	if err := os.MkdirAll(filepath.Dir(sidecarPath), constants.DefaultPerms755); err != nil {
		return err
	}

	// only apply the version on a write
	sc.Version = constants.SidecarVersion
	scBytes, err := json.MarshalIndent(sc, "", "    ")
	if err != nil {
		return err
	}

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

func (app *Avalanche) UpdateSidecar(sc *models.Sidecar) error {
	sc.Version = constants.SidecarVersion
	scBytes, err := json.MarshalIndent(sc, "", "    ")
	if err != nil {
		return err
	}

	sidecarPath := app.GetSidecarPath(sc.Name)
	return os.WriteFile(sidecarPath, scBytes, WriteReadReadPerms)
}

func (app *Avalanche) UpdateSidecarNetworks(
	sc *models.Sidecar,
	network models.Network,
	subnetID ids.ID,
	blockchainID ids.ID,
) error {
	if sc.Networks == nil {
		sc.Networks = make(map[string]models.NetworkData)
	}
	sc.Networks[network.String()] = models.NetworkData{
		SubnetID:     subnetID,
		BlockchainID: blockchainID,
	}
	if err := app.UpdateSidecar(sc); err != nil {
		return fmt.Errorf("creation of chains and subnet was successful, but failed to update sidecar: %w", err)
	}
	return nil
}

func (app *Avalanche) GetTokenName(subnetName string) string {
	sidecar, err := app.LoadSidecar(subnetName)
	if err != nil {
		return constants.DefaultTokenName
	}
	return sidecar.TokenName
}

func (app *Avalanche) GetSidecarNames() ([]string, error) {
	matches, err := os.ReadDir(app.GetSubnetDir())
	if err != nil {
		return nil, err
	}

	var names []string
	for _, m := range matches {
		// a subnet dir could theoretically exist without a sidecar yet...
		if _, err := os.Stat(filepath.Join(app.GetSubnetDir(), m.Name(), constants.SidecarFileName)); err == nil {
			names = append(names, m.Name())
		}
	}
	return names, nil
}
