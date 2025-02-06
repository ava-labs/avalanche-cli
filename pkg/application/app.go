// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package application

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ava-labs/apm/apm"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/monitoring"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"

	"golang.org/x/exp/maps"
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

func (app *Avalanche) GetRunFile(prefix string) string {
	return filepath.Join(app.GetRunDir(), prefix+constants.ServerRunFile)
}

func (app *Avalanche) GetSnapshotsDir() string {
	return filepath.Join(app.baseDir, constants.SnapshotsDirName)
}

func (app *Avalanche) GetSnapshotPath(snapshotName string) string {
	return filepath.Join(app.GetSnapshotsDir(), "anr-snapshot-"+snapshotName)
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

func (app *Avalanche) GetServicesDir(baseDir string) string {
	if baseDir == "" {
		baseDir = app.baseDir
	}
	return filepath.Join(baseDir, constants.ServicesDir)
}

func (app *Avalanche) GetCustomVMDir() string {
	return filepath.Join(app.baseDir, constants.CustomVMDir)
}

func (app *Avalanche) GetPluginsDir() string {
	return filepath.Join(app.baseDir, constants.PluginDir)
}

func (app *Avalanche) GetLocalDir(clusterName string) string {
	return filepath.Join(app.baseDir, constants.LocalDir, clusterName)
}

func (app *Avalanche) GetLogDir() string {
	return filepath.Join(app.baseDir, constants.LogDir)
}

func (app *Avalanche) GetAggregatorLogDir(clusterName string) string {
	if clusterName != "" {
		conf, err := app.GetClusterConfig(clusterName)
		if err == nil && conf.Local {
			return app.GetLocalDir(clusterName)
		}
	}
	return app.GetLogDir()
}

// Remove all plugins from plugin dir
func (app *Avalanche) ResetPluginsDir() error {
	pluginDir := app.GetPluginsDir()
	installedPlugins, err := os.ReadDir(pluginDir)
	if err != nil {
		return err
	}
	for _, plugin := range installedPlugins {
		if err = os.Remove(filepath.Join(pluginDir, plugin.Name())); err != nil {
			return err
		}
	}
	return nil
}

func (app *Avalanche) GetAvalanchegoBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.AvalancheGoInstallDir)
}

func (app *Avalanche) GetICMContractsBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.ICMContractsInstallDir)
}

func (app *Avalanche) GetICMRelayerBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.ICMRelayerInstallDir)
}

func (app *Avalanche) GetLocalRelayerDir(networkKind models.NetworkKind) string {
	networkDirName := strings.ReplaceAll(networkKind.String(), " ", "")
	return filepath.Join(app.GetRunDir(), networkDirName, constants.LocalRelayerDir)
}

func (app *Avalanche) GetLocalRelayerStorageDir(networkKind models.NetworkKind) string {
	return filepath.Join(app.GetLocalRelayerDir(networkKind), constants.ICMRelayerStorageDir)
}

func (app *Avalanche) GetLocalRelayerConfigPath(networkKind models.NetworkKind, localNetworkRootDir string) string {
	if localNetworkRootDir != "" {
		return filepath.Join(localNetworkRootDir, constants.ICMRelayerConfigFilename)
	}
	return filepath.Join(app.GetLocalRelayerDir(networkKind), constants.ICMRelayerConfigFilename)
}

func (app *Avalanche) GetLocalRelayerLogPath(networkKind models.NetworkKind) string {
	return filepath.Join(app.GetLocalRelayerDir(networkKind), constants.ICMRelayerLogFilename)
}

func (app *Avalanche) GetLocalRelayerRunPath(networkKind models.NetworkKind) string {
	return filepath.Join(app.GetLocalRelayerDir(networkKind), constants.ICMRelayerRunFilename)
}

func (app *Avalanche) GetICMRelayerServiceDir(baseDir string) string {
	return filepath.Join(app.GetServicesDir(baseDir), constants.ICMRelayerInstallDir)
}

func (app *Avalanche) GetICMRelayerServiceConfigPath(baseDir string) string {
	return filepath.Join(app.GetICMRelayerServiceDir(baseDir), constants.ICMRelayerConfigFilename)
}

func (app *Avalanche) GetICMRelayerServiceStorageDir(baseDir string) string {
	if baseDir != "" {
		return filepath.Join(baseDir, constants.ICMRelayerStorageDir)
	}
	return filepath.Join(app.GetICMRelayerServiceDir(""), constants.ICMRelayerStorageDir)
}

func (app *Avalanche) GetSubnetEVMBinDir() string {
	return filepath.Join(app.baseDir, constants.AvalancheCliBinDir, constants.SubnetEVMInstallDir)
}

func (app *Avalanche) GetUpgradeBytesFilepath(blockchainName string) string {
	return filepath.Join(app.GetSubnetDir(), blockchainName, constants.UpgradeFileName)
}

func (app *Avalanche) GetCustomVMPath(blockchainName string) string {
	return filepath.Join(app.GetCustomVMDir(), blockchainName)
}

func (app *Avalanche) GetAPMVMPath(vmid string) string {
	return filepath.Join(app.GetAPMPluginDir(), vmid)
}

func (app *Avalanche) GetGenesisPath(blockchainName string) string {
	return filepath.Join(app.GetSubnetDir(), blockchainName, constants.GenesisFileName)
}

func (app *Avalanche) GetAvagoNodeConfigPath(blockchainName string) string {
	return filepath.Join(app.GetSubnetDir(), blockchainName, constants.NodeConfigFileName)
}

func (app *Avalanche) GetChainConfigPath(blockchainName string) string {
	return filepath.Join(app.GetSubnetDir(), blockchainName, constants.ChainConfigFileName)
}

func (app *Avalanche) GetAvagoSubnetConfigPath(blockchainName string) string {
	return filepath.Join(app.GetSubnetDir(), blockchainName, constants.SubnetConfigFileName)
}

func (app *Avalanche) GetSidecarPath(blockchainName string) string {
	return filepath.Join(app.GetSubnetDir(), blockchainName, constants.SidecarFileName)
}

func (app *Avalanche) GetNodeConfigPath(nodeName string) string {
	return filepath.Join(app.GetNodesDir(), nodeName, constants.NodeCloudConfigFileName)
}

func (app *Avalanche) GetNodeInstanceDirPath(nodeName string) string {
	return filepath.Join(app.GetNodesDir(), nodeName)
}

func (app *Avalanche) GetNodeStakingDir(nodeIP string) string {
	return filepath.Join(app.GetNodesDir(), constants.StakingDir, nodeIP)
}

func (app *Avalanche) GetNodeInstanceAvaGoConfigDirPath(nodeName string) string {
	return filepath.Join(app.GetAnsibleDir(), nodeName)
}

func (app *Avalanche) GetAnsibleDir() string {
	return filepath.Join(app.GetNodesDir(), constants.AnsibleDir)
}

func (app *Avalanche) GetMonitoringDir() string {
	return filepath.Join(app.GetNodesDir(), constants.MonitoringDir)
}

func (app *Avalanche) GetMonitoringInventoryDir(clusterName string) string {
	return filepath.Join(app.GetAnsibleInventoryDirPath(clusterName), constants.MonitoringDir)
}

func (app *Avalanche) GetLoadTestInventoryDir(clusterName string) string {
	return filepath.Join(app.GetAnsibleInventoryDirPath(clusterName), constants.LoadTestDir)
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

func (app *Avalanche) GetClustersConfigPath() string {
	return filepath.Join(app.GetNodesDir(), constants.ClustersConfigFileName)
}

func (app *Avalanche) GetNodeBLSSecretKeyPath(instanceID string) string {
	return filepath.Join(app.GetNodeInstanceDirPath(instanceID), constants.BLSKeyFileName)
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
	return filepath.Join(app.GetLogDir(), constants.APMLogName)
}

func (app *Avalanche) GetAPMPluginDir() string {
	return filepath.Join(app.baseDir, constants.APMPluginDir)
}

func (app *Avalanche) GetKeyPath(keyName string) string {
	return filepath.Join(app.baseDir, constants.KeyDir, keyName+constants.KeySuffix)
}

func (app *Avalanche) GetKey(keyName string, network models.Network, createIfMissing bool) (*key.SoftKey, error) {
	if keyName == "ewoq" {
		return key.LoadEwoq(network.ID)
	} else {
		if createIfMissing {
			return key.LoadSoftOrCreate(network.ID, app.GetKeyPath(keyName))
		} else {
			return key.LoadSoft(network.ID, app.GetKeyPath(keyName))
		}
	}
}

func (app *Avalanche) GetUpgradeBytesFilePath(blockchainName string) string {
	return filepath.Join(app.GetSubnetDir(), blockchainName, constants.UpgradeFileName)
}

func (app *Avalanche) GetDownloader() Downloader {
	return app.Downloader
}

func (*Avalanche) GetAvalanchegoCompatibilityURL() string {
	return constants.AvalancheGoCompatibilityURL
}

func (app *Avalanche) ReadUpgradeFile(blockchainName string) ([]byte, error) {
	upgradeBytesFilePath := app.GetUpgradeBytesFilePath(blockchainName)

	return app.readFile(upgradeBytesFilePath)
}

func (app *Avalanche) ReadLockUpgradeFile(blockchainName string) ([]byte, error) {
	upgradeBytesLockFilePath := app.GetUpgradeBytesFilePath(blockchainName) + constants.UpgradeBytesLockExtension

	return app.readFile(upgradeBytesLockFilePath)
}

func (app *Avalanche) WriteUpgradeFile(blockchainName string, bytes []byte) error {
	upgradeBytesFilePath := app.GetUpgradeBytesFilePath(blockchainName)

	return app.writeFile(upgradeBytesFilePath, bytes)
}

func (app *Avalanche) WriteLockUpgradeFile(blockchainName string, bytes []byte) error {
	upgradeBytesLockFilePath := app.GetUpgradeBytesFilePath(blockchainName) + constants.UpgradeBytesLockExtension

	return app.writeFile(upgradeBytesLockFilePath, bytes)
}

func (app *Avalanche) WriteGenesisFile(blockchainName string, genesisBytes []byte) error {
	genesisPath := app.GetGenesisPath(blockchainName)

	return app.writeFile(genesisPath, genesisBytes)
}

func (app *Avalanche) WriteAvagoNodeConfigFile(blockchainName string, bs []byte) error {
	path := app.GetAvagoNodeConfigPath(blockchainName)
	return app.writeFile(path, bs)
}

func (app *Avalanche) WriteChainConfigFile(blockchainName string, bs []byte) error {
	path := app.GetChainConfigPath(blockchainName)
	return app.writeFile(path, bs)
}

func (app *Avalanche) WriteAvagoSubnetConfigFile(blockchainName string, bs []byte) error {
	path := app.GetAvagoSubnetConfigPath(blockchainName)
	return app.writeFile(path, bs)
}

func (app *Avalanche) WriteNetworkUpgradesFile(blockchainName string, bs []byte) error {
	path := app.GetUpgradeBytesFilepath(blockchainName)
	return app.writeFile(path, bs)
}

func (app *Avalanche) GenesisExists(blockchainName string) bool {
	genesisPath := app.GetGenesisPath(blockchainName)
	_, err := os.Stat(genesisPath)
	return err == nil
}

func (app *Avalanche) AvagoNodeConfigExists(blockchainName string) bool {
	path := app.GetAvagoNodeConfigPath(blockchainName)
	_, err := os.Stat(path)
	return err == nil
}

func (app *Avalanche) ChainConfigExists(blockchainName string) bool {
	path := app.GetChainConfigPath(blockchainName)
	_, err := os.Stat(path)
	return err == nil
}

func (app *Avalanche) AvagoSubnetConfigExists(blockchainName string) bool {
	path := app.GetAvagoSubnetConfigPath(blockchainName)
	_, err := os.Stat(path)
	return err == nil
}

func (app *Avalanche) NetworkUpgradeExists(blockchainName string) bool {
	path := app.GetUpgradeBytesFilepath(blockchainName)
	_, err := os.Stat(path)
	return err == nil
}

func (app *Avalanche) ClustersConfigExists() bool {
	_, err := os.Stat(app.GetClustersConfigPath())
	return err == nil
}

func (app *Avalanche) SidecarExists(blockchainName string) bool {
	sidecarPath := app.GetSidecarPath(blockchainName)
	_, err := os.Stat(sidecarPath)
	return err == nil
}

func (app *Avalanche) BlockchainConfigExists(blockchainName string) bool {
	// There's always a sidecar, but imported blockchains don't have a genesis right now
	return app.SidecarExists(blockchainName)
}

func (app *Avalanche) KeyExists(keyName string) bool {
	keyPath := app.GetKeyPath(keyName)
	_, err := os.Stat(keyPath)
	return err == nil
}

func (app *Avalanche) CopyGenesisFile(inputFilename string, blockchainName string) error {
	genesisBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	genesisPath := app.GetGenesisPath(blockchainName)
	if err := os.MkdirAll(filepath.Dir(genesisPath), constants.DefaultPerms755); err != nil {
		return err
	}

	return os.WriteFile(genesisPath, genesisBytes, constants.WriteReadReadPerms)
}

func (app *Avalanche) CopyVMBinary(inputFilename string, blockchainName string) error {
	vmBytes, err := os.ReadFile(inputFilename)
	if err != nil {
		return err
	}
	vmPath := app.GetCustomVMPath(blockchainName)
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

func (app *Avalanche) HasSubnetEVMGenesis(blockchainName string) (bool, error, error) {
	if _, err := app.LoadRawGenesis(blockchainName); err != nil {
		return false, nil, err
	}
	// from here, we are sure to have a genesis file
	_, err := app.LoadEvmGenesis(blockchainName)
	if err != nil {
		return false, err, nil
	}
	return true, nil, nil
}

func (app *Avalanche) LoadEvmGenesis(blockchainName string) (core.Genesis, error) {
	genesisPath := app.GetGenesisPath(blockchainName)
	bs, err := os.ReadFile(genesisPath)
	if err != nil {
		return core.Genesis{}, err
	}
	return utils.ByteSliceToSubnetEvmGenesis(bs)
}

func (app *Avalanche) LoadRawGenesis(blockchainName string) ([]byte, error) {
	genesisPath := app.GetGenesisPath(blockchainName)
	return os.ReadFile(genesisPath)
}

func (app *Avalanche) LoadRawAvagoNodeConfig(blockchainName string) ([]byte, error) {
	return os.ReadFile(app.GetAvagoNodeConfigPath(blockchainName))
}

func (app *Avalanche) LoadRawChainConfig(blockchainName string) ([]byte, error) {
	return os.ReadFile(app.GetChainConfigPath(blockchainName))
}

func (app *Avalanche) LoadRawAvagoSubnetConfig(blockchainName string) ([]byte, error) {
	return os.ReadFile(app.GetAvagoSubnetConfigPath(blockchainName))
}

func (app *Avalanche) LoadRawNetworkUpgrades(blockchainName string) ([]byte, error) {
	return os.ReadFile(app.GetUpgradeBytesFilepath(blockchainName))
}

func (app *Avalanche) CreateSidecar(sc *models.Sidecar) error {
	if sc.TokenName == "" {
		sc.TokenName = constants.DefaultTokenName
		sc.TokenSymbol = constants.DefaultTokenSymbol
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

func (app *Avalanche) LoadSidecar(blockchainName string) (models.Sidecar, error) {
	if !app.SidecarExists(blockchainName) {
		return models.Sidecar{}, fmt.Errorf("subnet %q does not exist", blockchainName)
	}

	sidecarPath := app.GetSidecarPath(blockchainName)
	jsonBytes, err := os.ReadFile(sidecarPath)
	if err != nil {
		return models.Sidecar{}, err
	}

	var sc models.Sidecar
	err = json.Unmarshal(jsonBytes, &sc)

	if sc.TokenName == "" {
		sc.TokenName = constants.DefaultTokenName
		sc.TokenSymbol = constants.DefaultTokenSymbol
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
	icmMessengerAddress string,
	icmRegistryAddress string,
	bootstrapValidators []models.SubnetValidator,
	clusterName string,
	validatorManagerAddressStr string,
) error {
	if sc.Networks == nil {
		sc.Networks = make(map[string]models.NetworkData)
	}
	sc.Networks[network.Name()] = models.NetworkData{
		SubnetID:                   subnetID,
		BlockchainID:               blockchainID,
		RPCVersion:                 sc.RPCVersion,
		TeleporterMessengerAddress: icmMessengerAddress,
		TeleporterRegistryAddress:  icmRegistryAddress,
		BootstrapValidators:        bootstrapValidators,
		ClusterName:                clusterName,
	}
	if sc.Sovereign {
		sc.UpdateValidatorManagerAddress(network.Name(), validatorManagerAddressStr)
	}
	if err := app.UpdateSidecar(sc); err != nil {
		return fmt.Errorf("creation of blockchain was successful, but failed to update sidecar: %w", err)
	}
	return nil
}

func (app *Avalanche) GetTokenName(blockchainName string) string {
	sidecar, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return constants.DefaultTokenName
	}
	return sidecar.TokenName
}

func (app *Avalanche) GetTokenSymbol(blockchainName string) string {
	sidecar, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return constants.DefaultTokenSymbol
	}
	return sidecar.TokenSymbol
}

func (app *Avalanche) GetBlockchainNames() ([]string, error) {
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

func (app *Avalanche) GetBlockchainNamesOnNetwork(
	network models.Network,
	onlySOV bool,
) ([]string, error) {
	blockchainNames, err := app.GetBlockchainNames()
	if err != nil {
		return nil, err
	}
	filtered := []string{}
	for _, blockchainName := range blockchainNames {
		sc, err := app.LoadSidecar(blockchainName)
		if err != nil {
			return nil, err
		}
		networkName := network.Name()
		if sc.Networks[networkName].BlockchainID == ids.Empty {
			for k := range sc.Networks {
				sidecarNetwork, err := app.GetNetworkFromSidecarNetworkName(k)
				if err == nil {
					if sidecarNetwork.Kind == network.Kind && sidecarNetwork.Endpoint == network.Endpoint {
						networkName = sidecarNetwork.Name()
					}
				}
			}
		}
		sovKindCriteria := !onlySOV || onlySOV && sc.Sovereign
		if sc.Networks[networkName].BlockchainID != ids.Empty && sovKindCriteria {
			filtered = append(filtered, blockchainName)
		}
	}
	return filtered, nil
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

func (app *Avalanche) LoadClustersConfig() (models.ClustersConfig, error) {
	clustersConfigPath := app.GetClustersConfigPath()
	if !utils.FileExists(clustersConfigPath) {
		return models.ClustersConfig{
			Clusters: map[string]models.ClusterConfig{},
		}, nil
	}
	jsonBytes, err := os.ReadFile(clustersConfigPath)
	if err != nil {
		return models.ClustersConfig{}, err
	}
	var clustersConfig models.ClustersConfig
	var clustersConfigMap map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &clustersConfigMap); err != nil {
		return models.ClustersConfig{}, err
	}
	v, ok := clustersConfigMap["Version"]
	if !ok {
		// backwards compatibility V0
		var clustersConfigV0 models.ClustersConfigV0
		if err := json.Unmarshal(jsonBytes, &clustersConfigV0); err != nil {
			return models.ClustersConfig{}, err
		}
		clustersConfig.Version = constants.ClustersConfigVersion
		clustersConfig.KeyPair = clustersConfigV0.KeyPair
		clustersConfig.GCPConfig = clustersConfigV0.GCPConfig
		clustersConfig.Clusters = map[string]models.ClusterConfig{}
		for clusterName, nodes := range clustersConfigV0.Clusters {
			clustersConfig.Clusters[clusterName] = models.ClusterConfig{
				Nodes:   nodes,
				Network: models.NewFujiNetwork(),
			}
		}
		return clustersConfig, err
	}
	if v == constants.ClustersConfigVersion {
		if err := json.Unmarshal(jsonBytes, &clustersConfig); err != nil {
			return models.ClustersConfig{}, err
		}
		return clustersConfig, err
	}
	return models.ClustersConfig{}, fmt.Errorf("unsupported clusters config version %s", v)
}

func (app *Avalanche) GetClustersConfig() (models.ClustersConfig, error) {
	if app.ClustersConfigExists() {
		return app.LoadClustersConfig()
	}
	return models.ClustersConfig{}, nil
}

func (app *Avalanche) WriteClustersConfigFile(clustersConfig *models.ClustersConfig) error {
	clustersConfigPath := app.GetClustersConfigPath()
	if err := os.MkdirAll(filepath.Dir(clustersConfigPath), constants.DefaultPerms755); err != nil {
		return err
	}

	clustersConfig.Version = constants.ClustersConfigVersion
	clustersConfigBytes, err := json.MarshalIndent(clustersConfig, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(clustersConfigPath, clustersConfigBytes, constants.WriteReadReadPerms)
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

func (app *Avalanche) CreateMonitoringDir() error {
	monitoringDir := app.GetMonitoringDir()
	if !sdkutils.DirExists(monitoringDir) {
		err := os.MkdirAll(monitoringDir, constants.DefaultPerms755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *Avalanche) CreateMonitoringDashboardDir() error {
	monitoringDashboardDir := app.GetMonitoringDashboardDir()
	if !sdkutils.DirExists(monitoringDashboardDir) {
		err := os.MkdirAll(monitoringDashboardDir, constants.DefaultPerms755)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *Avalanche) GetAnsibleInventoryDirPath(clusterName string) string {
	return filepath.Join(app.GetNodesDir(), constants.AnsibleInventoryDir, clusterName)
}

// CreateAnsibleNodeConfigDir creates the ansible node config directory specific for nodeID inside .avalanche-cli
func (app *Avalanche) CreateAnsibleNodeConfigDir(nodeID string) error {
	return os.MkdirAll(filepath.Join(app.GetAnsibleDir(), nodeID), constants.DefaultPerms755)
}

func (app *Avalanche) GetNodeConfigJSONFile(nodeID string) string {
	return filepath.Join(app.GetAnsibleDir(), nodeID, constants.NodeConfigJSONFile)
}

func (app *Avalanche) GetClusterYAMLFilePath(clusterName string) string {
	return filepath.Join(app.GetAnsibleInventoryDirPath(clusterName), constants.ClusterYAMLFileName)
}

func (app *Avalanche) GetMonitoringDashboardDir() string {
	return filepath.Join(app.GetMonitoringDir(), constants.DashboardsDir)
}

func (app *Avalanche) SetupMonitoringEnv() error {
	err := os.RemoveAll(app.GetMonitoringDir())
	if err != nil {
		return err
	}
	err = app.CreateMonitoringDir()
	if err != nil {
		return err
	}
	err = app.CreateMonitoringDashboardDir()
	if err != nil {
		return err
	}
	return monitoring.Setup(app.GetMonitoringDir())
}

func (app *Avalanche) ClusterExists(clusterName string) (bool, error) {
	clustersConfig, err := app.GetClustersConfig()
	if err != nil {
		return false, err
	}
	_, ok := clustersConfig.Clusters[clusterName]
	return ok, nil
}

func (app *Avalanche) GetClusterConfig(clusterName string) (models.ClusterConfig, error) {
	if exists, err := app.ClusterExists(clusterName); err != nil {
		return models.ClusterConfig{}, err
	} else if !exists {
		return models.ClusterConfig{}, fmt.Errorf("cluster does not exists")
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return models.ClusterConfig{}, err
	}
	clusterConfig := clustersConfig.Clusters[clusterName]
	clusterConfig.Network = models.NewNetworkFromCluster(clusterConfig.Network, clusterName)
	return clusterConfig, nil
}

func (app *Avalanche) SetClusterConfig(clusterName string, clusterConfig models.ClusterConfig) error {
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return err
	}
	clustersConfig.Clusters[clusterName] = clusterConfig
	return app.WriteClustersConfigFile(&clustersConfig)
}

func (app *Avalanche) GetClusterNetwork(clusterName string) (models.Network, error) {
	clusterConfig, err := app.GetClusterConfig(clusterName)
	if err != nil {
		return models.UndefinedNetwork, err
	}
	return clusterConfig.Network, nil
}

func (app *Avalanche) ListClusterNames() ([]string, error) {
	if !app.ClustersConfigExists() {
		return []string{}, nil
	}
	clustersConfig, err := app.LoadClustersConfig()
	if err != nil {
		return []string{}, err
	}
	return maps.Keys(clustersConfig.Clusters), nil
}

func (app *Avalanche) GetNetworkFromSidecarNetworkName(
	networkName string,
) (models.Network, error) {
	switch {
	case networkName == models.Local.String():
		return models.NewLocalNetwork(), nil
	case strings.HasPrefix(networkName, "Cluster"):
		// network names on sidecar can refer to a cluster in the form "Cluster <clusterName>"
		// we use clusterName to find out the underlying network for the cluster
		// (one of local, devnet, fuji, mainnet)
		parts := strings.Split(networkName, " ")
		if len(parts) != 2 {
			return models.UndefinedNetwork, fmt.Errorf("expected 'Cluster clusterName' on network name %s", networkName)
		}
		return app.GetClusterNetwork(parts[1])
	case networkName == models.Fuji.String():
		return models.NewFujiNetwork(), nil
	case networkName == models.Mainnet.String():
		return models.NewMainnetNetwork(), nil
	}
	return models.UndefinedNetwork, fmt.Errorf("unsupported network name")
}
