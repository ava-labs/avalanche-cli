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

func (app *Avalanche) GetNodesDir() string {
	return filepath.Join(app.baseDir, constants.NodesDir)
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

func (app *Avalanche) GetPluginsDir() string {
	return filepath.Join(app.baseDir, constants.PluginDir)
}

func (app *Avalanche) GetAvalanchegoBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.AvalancheGoInstallDir)
}

func (app *Avalanche) GetSubnetEVMBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.SubnetEVMInstallDir)
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

func (app *Avalanche) GetAvagoNodeConfigPath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.NodeConfigFileName)
}

func (app *Avalanche) GetChainConfigPath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.ChainConfigFileName)
}

func (app *Avalanche) GetAvagoSubnetConfigPath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.SubnetConfigFileName)
}

func (app *Avalanche) GetGenesisMainnetPath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.GenesisMainnetFileName)
}

func (app *Avalanche) GetSidecarPath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.SidecarFileName)
}

func (app *Avalanche) GetConfigPath() string {
	return filepath.Join(app.baseDir, constants.ConfigDir)
}

func (app *Avalanche) GetNodeConfigPath(nodeName string) string {
	return filepath.Join(app.GetNodesDir(), nodeName, constants.NodeCloudConfigFileName)
}

func (app *Avalanche) GetNodeInstanceDirPath(nodeName string) string {
	return filepath.Join(app.GetNodesDir(), nodeName)
}

func (app *Avalanche) GetAnsibleDir() string {
	return filepath.Join(app.GetNodesDir(), constants.AnsibleDir)
}

func (app *Avalanche) CreateAnsibleDir() error {
	ansibleDir := app.GetAnsibleDir()
	if _, err := os.Stat(ansibleDir); os.IsNotExist(err) {
		err = os.Mkdir(ansibleDir, constants.DefaultPerms755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *Avalanche) CreateTerraformDir() error {
	nodesDir := app.GetNodesDir()
	if _, err := os.Stat(nodesDir); os.IsNotExist(err) {
		err = os.Mkdir(nodesDir, constants.DefaultPerms755)
		if err != nil {
			return err
		}
	}
	nodeTerraformDir := app.GetTerraformDir()
	if _, err := os.Stat(nodeTerraformDir); os.IsNotExist(err) {
		err = os.Mkdir(nodeTerraformDir, constants.DefaultPerms755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *Avalanche) CreateAnsibleInventoryDir() error {
	inventoriesDir := filepath.Join(app.GetNodesDir(), constants.AnsibleInventoryDir)
	if _, err := os.Stat(inventoriesDir); os.IsNotExist(err) {
		err = os.Mkdir(inventoriesDir, constants.DefaultPerms755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *Avalanche) CreateAnsiblePlaybookDir() error {
	playbookDir := filepath.Join(app.GetAnsibleDir(), constants.AnsiblePlaybookDir)
	if _, err := os.Stat(playbookDir); os.IsNotExist(err) {
		err = os.Mkdir(playbookDir, constants.DefaultPerms755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *Avalanche) GetTerraformDir() string {
	return filepath.Join(app.GetNodesDir(), constants.TerraformDir)
}

func (app *Avalanche) GetTempCertPath(certName string) string {
	return filepath.Join(app.GetTerraformDir(), certName)
}

func (app *Avalanche) GetClusterConfigPath() string {
	return filepath.Join(app.GetNodesDir(), constants.ClusterConfigFileName)
}

func (app *Avalanche) GetNodeBLSSecretKeyPath(instanceID string) string {
	return filepath.Join(app.GetNodeInstanceDirPath(instanceID), constants.BLSKeyFileName)
}

func (app *Avalanche) GetElasticSubnetConfigPath(subnetName string) string {
	return filepath.Join(app.GetSubnetDir(), subnetName, constants.ElasticSubnetConfigFileName)
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

func (app *Avalanche) ReadUpgradeFile(subnetName string) ([]byte, error) {
	upgradeBytesFilePath := app.GetUpgradeBytesFilePath(subnetName)

	return app.readFile(upgradeBytesFilePath)
}

func (app *Avalanche) ReadLockUpgradeFile(subnetName string) ([]byte, error) {
	upgradeBytesLockFilePath := app.GetUpgradeBytesFilePath(subnetName) + constants.UpgradeBytesLockExtension

	return app.readFile(upgradeBytesLockFilePath)
}

func (app *Avalanche) WriteUpgradeFile(subnetName string, bytes []byte) error {
	upgradeBytesFilePath := app.GetUpgradeBytesFilePath(subnetName)

	return app.writeFile(upgradeBytesFilePath, bytes)
}

func (app *Avalanche) WriteLockUpgradeFile(subnetName string, bytes []byte) error {
	upgradeBytesLockFilePath := app.GetUpgradeBytesFilePath(subnetName) + constants.UpgradeBytesLockExtension

	return app.writeFile(upgradeBytesLockFilePath, bytes)
}

func (app *Avalanche) WriteGenesisFile(subnetName string, genesisBytes []byte) error {
	genesisPath := app.GetGenesisPath(subnetName)

	return app.writeFile(genesisPath, genesisBytes)
}

func (app *Avalanche) WriteGenesisMainnetFile(subnetName string, genesisBytes []byte) error {
	genesisPath := app.GetGenesisMainnetPath(subnetName)

	return app.writeFile(genesisPath, genesisBytes)
}

func (app *Avalanche) WriteAvagoNodeConfigFile(subnetName string, bs []byte) error {
	path := app.GetAvagoNodeConfigPath(subnetName)
	return app.writeFile(path, bs)
}

func (app *Avalanche) WriteChainConfigFile(subnetName string, bs []byte) error {
	path := app.GetChainConfigPath(subnetName)
	return app.writeFile(path, bs)
}

func (app *Avalanche) WriteAvagoSubnetConfigFile(subnetName string, bs []byte) error {
	path := app.GetAvagoSubnetConfigPath(subnetName)
	return app.writeFile(path, bs)
}

func (app *Avalanche) WriteNetworkUpgradesFile(subnetName string, bs []byte) error {
	path := app.GetUpgradeBytesFilepath(subnetName)
	return app.writeFile(path, bs)
}

func (app *Avalanche) GenesisExists(subnetName string) bool {
	genesisPath := app.GetGenesisPath(subnetName)
	_, err := os.Stat(genesisPath)
	return err == nil
}

func (app *Avalanche) AvagoNodeConfigExists(subnetName string) bool {
	path := app.GetAvagoNodeConfigPath(subnetName)
	_, err := os.Stat(path)
	return err == nil
}

func (app *Avalanche) ChainConfigExists(subnetName string) bool {
	path := app.GetChainConfigPath(subnetName)
	_, err := os.Stat(path)
	return err == nil
}

func (app *Avalanche) AvagoSubnetConfigExists(subnetName string) bool {
	path := app.GetAvagoSubnetConfigPath(subnetName)
	_, err := os.Stat(path)
	return err == nil
}

func (app *Avalanche) NetworkUpgradeExists(subnetName string) bool {
	path := app.GetUpgradeBytesFilepath(subnetName)
	_, err := os.Stat(path)
	return err == nil
}

func (app *Avalanche) ClusterConfigExists() bool {
	_, err := os.Stat(app.GetClusterConfigPath())
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

	return os.WriteFile(genesisPath, genesisBytes, constants.WriteReadReadPerms)
}

func (app *Avalanche) CopyVMBinary(inputFilename string, subnetName string) error {
	vmBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	vmPath := app.GetCustomVMPath(subnetName)
	return os.WriteFile(vmPath, vmBytes, constants.DefaultPerms755)
}

func (app *Avalanche) CopyKeyFile(inputFilename string, keyName string) error {
	keyBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	keyPath := app.GetKeyPath(keyName)
	return os.WriteFile(keyPath, keyBytes, constants.WriteReadReadPerms)
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

func (app *Avalanche) LoadRawGenesis(subnetName string, network models.Network) ([]byte, error) {
	genesisPath := app.GetGenesisPath(subnetName)
	genesisBytes, err := os.ReadFile(genesisPath)
	if err != nil {
		return nil, err
	}
	if network == models.Mainnet {
		genesisPath = app.GetGenesisMainnetPath(subnetName)
		genesisMainnetBytes, err := os.ReadFile(genesisPath)
		if err == nil {
			genesisBytes = genesisMainnetBytes
		}
	}
	return genesisBytes, err
}

func (app *Avalanche) LoadRawAvagoNodeConfig(subnetName string) ([]byte, error) {
	return os.ReadFile(app.GetAvagoNodeConfigPath(subnetName))
}

func (app *Avalanche) LoadRawChainConfig(subnetName string) ([]byte, error) {
	return os.ReadFile(app.GetChainConfigPath(subnetName))
}

func (app *Avalanche) LoadRawAvagoSubnetConfig(subnetName string) ([]byte, error) {
	return os.ReadFile(app.GetAvagoSubnetConfigPath(subnetName))
}

func (app *Avalanche) LoadRawNetworkUpgrades(subnetName string) ([]byte, error) {
	return os.ReadFile(app.GetUpgradeBytesFilepath(subnetName))
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

	return os.WriteFile(sidecarPath, scBytes, constants.WriteReadReadPerms)
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
	return os.WriteFile(sidecarPath, scBytes, constants.WriteReadReadPerms)
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
		RPCVersion:   sc.RPCVersion,
	}
	if err := app.UpdateSidecar(sc); err != nil {
		return fmt.Errorf("creation of chains and subnet was successful, but failed to update sidecar: %w", err)
	}
	return nil
}

func (app *Avalanche) UpdateSidecarElasticSubnet(
	sc *models.Sidecar,
	network models.Network,
	subnetID ids.ID,
	assetID ids.ID,
	pchainTXID ids.ID,
	tokenName string,
	tokenSymbol string,
) error {
	if sc.ElasticSubnet == nil {
		sc.ElasticSubnet = make(map[string]models.ElasticSubnet)
	}
	partialTxs := sc.ElasticSubnet[network.String()].Txs
	sc.ElasticSubnet[network.String()] = models.ElasticSubnet{
		SubnetID:    subnetID,
		AssetID:     assetID,
		PChainTXID:  pchainTXID,
		TokenName:   tokenName,
		TokenSymbol: tokenSymbol,
		Txs:         partialTxs,
	}
	if err := app.UpdateSidecar(sc); err != nil {
		return err
	}
	return nil
}

func (app *Avalanche) UpdateSidecarPermissionlessValidator(
	sc *models.Sidecar,
	network models.Network,
	nodeID string,
	txID ids.ID,
) error {
	elasticSubnet := sc.ElasticSubnet[network.String()]
	if elasticSubnet.Validators == nil {
		elasticSubnet.Validators = make(map[string]models.PermissionlessValidators)
	}
	elasticSubnet.Validators[nodeID] = models.PermissionlessValidators{TxID: txID}
	sc.ElasticSubnet[network.String()] = elasticSubnet
	if err := app.UpdateSidecar(sc); err != nil {
		return err
	}
	return nil
}

func (app *Avalanche) UpdateSidecarElasticSubnetPartialTx(
	sc *models.Sidecar,
	network models.Network,
	txName string,
	txID ids.ID,
) error {
	if sc.ElasticSubnet == nil {
		sc.ElasticSubnet = make(map[string]models.ElasticSubnet)
	}
	partialTxs := make(map[string]ids.ID)
	if sc.ElasticSubnet[network.String()].Txs != nil {
		partialTxs = sc.ElasticSubnet[network.String()].Txs
	}
	partialTxs[txName] = txID
	sc.ElasticSubnet[network.String()] = models.ElasticSubnet{
		Txs: partialTxs,
	}
	return app.UpdateSidecar(sc)
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
		if !m.IsDir() {
			continue
		}
		// a subnet dir could theoretically exist without a sidecar yet...
		if _, err := os.Stat(filepath.Join(app.GetSubnetDir(), m.Name(), constants.SidecarFileName)); err == nil {
			names = append(names, m.Name())
		}
	}
	return names, nil
}

func (*Avalanche) readFile(path string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), constants.DefaultPerms755); err != nil {
		return nil, err
	}

	return os.ReadFile(path)
}

func (*Avalanche) writeFile(path string, bytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), constants.DefaultPerms755); err != nil {
		return err
	}

	return os.WriteFile(path, bytes, constants.WriteReadReadPerms)
}

func (app *Avalanche) LoadConfig() (models.Config, error) {
	configPath := app.GetConfigPath()
	jsonBytes, err := os.ReadFile(configPath)
	if err != nil {
		return models.Config{}, err
	}

	var config models.Config
	err = json.Unmarshal(jsonBytes, &config)
	return config, err
}

func (app *Avalanche) ConfigFileExists() bool {
	configPath := app.GetConfigPath()
	_, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (app *Avalanche) WriteConfigFile(bytes []byte) error {
	configPath := app.GetConfigPath()
	return app.writeFile(configPath, bytes)
}

func (app *Avalanche) CreateNodeCloudConfigFile(nodeName string, nodeConfig *models.NodeConfig) error {
	nodeConfigPath := app.GetNodeConfigPath(nodeName)
	if err := os.MkdirAll(filepath.Dir(nodeConfigPath), constants.DefaultPerms755); err != nil {
		return err
	}

	esBytes, err := json.MarshalIndent(nodeConfig, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(nodeConfigPath, esBytes, constants.WriteReadReadPerms)
}

func (app *Avalanche) CreateElasticSubnetConfig(subnetName string, es *models.ElasticSubnetConfig) error {
	elasticSubetConfigPath := app.GetElasticSubnetConfigPath(subnetName)
	if err := os.MkdirAll(filepath.Dir(elasticSubetConfigPath), constants.DefaultPerms755); err != nil {
		return err
	}

	esBytes, err := json.MarshalIndent(es, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(elasticSubetConfigPath, esBytes, constants.WriteReadReadPerms)
}

func (app *Avalanche) LoadElasticSubnetConfig(subnetName string) (models.ElasticSubnetConfig, error) {
	elasticSubnetConfigPath := app.GetElasticSubnetConfigPath(subnetName)
	jsonBytes, err := os.ReadFile(elasticSubnetConfigPath)
	if err != nil {
		return models.ElasticSubnetConfig{}, err
	}

	var esc models.ElasticSubnetConfig
	err = json.Unmarshal(jsonBytes, &esc)

	return esc, err
}

func (app *Avalanche) LoadClusterNodeConfig(nodeName string) (models.NodeConfig, error) {
	nodeConfigPath := app.GetNodeConfigPath(nodeName)
	jsonBytes, err := os.ReadFile(nodeConfigPath)
	if err != nil {
		return models.NodeConfig{}, err
	}
	var nodeConfig models.NodeConfig
	err = json.Unmarshal(jsonBytes, &nodeConfig)
	return nodeConfig, err
}

func (app *Avalanche) LoadClusterConfig() (models.ClusterConfig, error) {
	clusterConfigPath := app.GetClusterConfigPath()
	jsonBytes, err := os.ReadFile(clusterConfigPath)
	if err != nil {
		return models.ClusterConfig{}, err
	}
	var clusterConfig models.ClusterConfig
	err = json.Unmarshal(jsonBytes, &clusterConfig)
	return clusterConfig, err
}

func (app *Avalanche) WriteClusterConfigFile(clusterConfig *models.ClusterConfig) error {
	clusterConfigPath := app.GetClusterConfigPath()
	if err := os.MkdirAll(filepath.Dir(clusterConfigPath), constants.DefaultPerms755); err != nil {
		return err
	}

	clusterConfigBytes, err := json.MarshalIndent(clusterConfig, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(clusterConfigPath, clusterConfigBytes, constants.WriteReadReadPerms)
}

func (*Avalanche) GetSSHCertFilePath(certName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".ssh", certName), nil
}

func (app *Avalanche) CheckCertInSSHDir(certName string) (bool, error) {
	certPath, err := app.GetSSHCertFilePath(certName)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(certPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (app *Avalanche) GetAnsibleInventoryDirPath(clusterName string) string {
	return filepath.Join(app.GetNodesDir(), constants.AnsibleInventoryDir, clusterName)
}

func (app *Avalanche) GetAnsibleStatusDir() string {
	return filepath.Join(app.GetAnsibleDir(), constants.AnsibleStatusDir)
}

func (app *Avalanche) GetBootstrappedJSONFile() string {
	return filepath.Join(app.GetAnsibleStatusDir(), constants.IsBootstrappedJSONFile)
}

func (app *Avalanche) GetAvalancheGoJSONFile() string {
	return filepath.Join(app.GetAnsibleStatusDir(), constants.AvalancheGoVersionJSONFile)
}

func (app *Avalanche) GetSubnetSyncJSONFile() string {
	return filepath.Join(app.GetAnsibleStatusDir(), constants.SubnetSyncJSONFile)
}

func (app *Avalanche) SetupAnsibleEnv() error {
	err := os.RemoveAll(app.GetAnsibleDir())
	if err != nil {
		return err
	}
	err = app.CreateAnsibleDir()
	if err != nil {
		return err
	}
	return app.CreateAnsiblePlaybookDir()
}

// CreateAnsibleStatusDir creates the ansible status directory inside .avalanche-cli
func (app *Avalanche) CreateAnsibleStatusDir() error {
	return os.MkdirAll(app.GetAnsibleStatusDir(), constants.DefaultPerms755)
}

// CreateAnsibleStatusFile creates file named fileName in .avalanche-cli ansible status directory
func (app *Avalanche) CreateAnsibleStatusFile(filePath string) error {
	if err := os.MkdirAll(app.GetAnsibleStatusDir(), constants.DefaultPerms755); err != nil {
		return err
	}
	statusFile, err := os.Create(filePath)
	if err != nil {
		return err
	}
	return statusFile.Close()
}

// RemoveAnsibleStatusDir deletes avalanche ansible status dir in .avalanche-cli
func (app *Avalanche) RemoveAnsibleStatusDir() error {
	return os.RemoveAll(app.GetAnsibleStatusDir())
}
