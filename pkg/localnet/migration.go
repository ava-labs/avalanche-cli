// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/interchain/relayer"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-network-runner/network"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"
	"github.com/ava-labs/avalanchego/chains"
	avagoconfig "github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"

	dircopy "github.com/otiai10/copy"
)

const migratedSuffix = "-migrated"

// Called from 'internal/migrations' previously to any command execution
// Iterates over all local clusters, finding if there is a legacy network runner one
// If that is the case, renames it with migratedSuffix, creates a new tmpnet cluster
// based on all network runner cluster info, and then remove the legacy one on success
// If the is a cluster running, first stops it, and any relayer associated with it,
// then migrate the cluster, then run thew new one again, together with the relayer
// Relayer stop/start is needed because a connected relayer makes bootstrapping to
// fail upon cluster restart
func MigrateANRToTmpNet(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
) error {
	ctx, cancel := sdkutils.GetTimedContext(3 * time.Minute)
	defer cancel()
	var (
		clusterToReload           string
		clusterToReloadNetwork    models.Network
		clusterToReloadHasRelayer bool
	)

	cli, _ := binutils.NewGRPCClientWithEndpoint(
		binutils.LocalClusterGRPCServerEndpoint,
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if cli != nil {
		// ANR is running
		status, _ := cli.Status(ctx)
		if status != nil && status.ClusterInfo != nil {
			// there is a local cluster up
			var err error
			clusterToReload = filepath.Base(status.ClusterInfo.RootDataDir)
			clusterToReloadNetwork = models.NetworkFromNetworkID(status.ClusterInfo.NetworkId)
			clusterToReloadHasRelayer, _, _, err = relayer.RelayerIsUp(app.GetLocalRelayerRunPath(clusterToReloadNetwork.Kind))
			if err != nil {
				return nil
			}
			if clusterToReloadHasRelayer {
				printFunc("Stopping relayer")
				if err := relayer.RelayerCleanup(
					app.GetLocalRelayerRunPath(clusterToReloadNetwork.Kind),
					app.GetLocalRelayerLogPath(clusterToReloadNetwork.Kind),
					app.GetLocalRelayerStorageDir(clusterToReloadNetwork.Kind),
				); err != nil {
					return err
				}
			}
			printFunc("Found running cluster %s. Will restart after migration.", clusterToReload)
			if _, err := cli.Stop(ctx); err != nil {
				return fmt.Errorf("failed to stop avalanchego: %w", err)
			}
		}
		if err := binutils.KillgRPCServerProcess(
			app,
			binutils.LocalClusterGRPCServerEndpoint,
			constants.ServerRunFileLocalClusterPrefix,
		); err != nil {
			return err
		}
	}
	toMigrate := []string{}
	clustersDir := app.GetLocalClustersDir()
	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return fmt.Errorf("failed to read local clusters dir %s: %w", clustersDir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		clusterName := entry.Name()
		if _, err := GetLocalCluster(app, clusterName); err != nil {
			// not tmpnet, or dir with failures
			networkDir := filepath.Join(clustersDir, clusterName)
			if strings.HasSuffix(clusterName, migratedSuffix) {
				printFunc("%s was partially migrated with failure. Please manually recover", networkDir)
				continue
			}
			jsonPath := filepath.Join(networkDir, "network.json")
			if utils.FileExists(jsonPath) {
				bs, err := os.ReadFile(jsonPath)
				if err != nil {
					printFunc("Failure loading JSON on cluster at %s: %s. Please manually recover", networkDir, err)
					continue
				}
				var config network.Config
				if err := json.Unmarshal(bs, &config); err != nil {
					printFunc("Unexpected JSON format on cluster at %s: %s. Please manually recover", networkDir, err)
					continue
				}
				toMigrate = append(toMigrate, clusterName)
			} else {
				printFunc("Unexpected format on cluster at %s. Please manually recover", networkDir)
			}
		}
	}
	for _, clusterName := range toMigrate {
		printFunc("Migrating %s", clusterName)
		if err := migrateCluster(app, printFunc, clusterName); err != nil {
			printFunc("Failure migrating %s at %s: %s", clusterName, GetLocalClusterDir(app, clusterName), err)
		}
	}
	if clusterToReload != "" {
		printFunc("Restarting cluster %s.", clusterToReload)
		if err := LoadLocalCluster(app, clusterToReload, ""); err != nil {
			return err
		}
		if clusterToReloadHasRelayer {
			localNetworkRootDir := ""
			if clusterToReloadNetwork.Kind == models.Local {
				localNetworkRootDir, err = GetLocalNetworkDir(app)
				if err != nil {
					return err
				}
			}
			relayerConfigPath := app.GetLocalRelayerConfigPath(clusterToReloadNetwork.Kind, localNetworkRootDir)
			relayerBinPath := ""
			if clusterToReloadNetwork.Kind == models.Local {
				if b, extraLocalNetworkData, err := GetExtraLocalNetworkData(app, ""); err != nil {
					return err
				} else if b {
					relayerBinPath = extraLocalNetworkData.RelayerPath
				}
			}
			if utils.FileExists(relayerConfigPath) {
				printFunc("Restarting relayer")
				if _, err := relayer.DeployRelayer(
					constants.DefaultRelayerVersion,
					relayerBinPath,
					app.GetICMRelayerBinDir(),
					relayerConfigPath,
					app.GetLocalRelayerLogPath(clusterToReloadNetwork.Kind),
					app.GetLocalRelayerRunPath(clusterToReloadNetwork.Kind),
					app.GetLocalRelayerStorageDir(clusterToReloadNetwork.Kind),
				); err != nil {
					return err
				}
			}
		}
	}
	if len(toMigrate) > 0 {
		printFunc("")
	}
	return nil
}

// Migrates a network runner cluster, by first renaming it, then creating
// a new tmpnet one based on it, then finally removing it on success
func migrateCluster(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	clusterName string,
) error {
	networkDir := GetLocalClusterDir(app, clusterName)
	anrDir := GetLocalClusterDir(app, clusterName+migratedSuffix)
	if err := os.Rename(networkDir, anrDir); err != nil {
		return err
	}
	jsonPath := filepath.Join(anrDir, "network.json")
	bs, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}
	var config network.Config
	if err := json.Unmarshal(bs, &config); err != nil {
		return err
	}
	connectionSettings := ConnectionSettings{
		NetworkID: config.NetworkID,
	}
	trackSubnetsStr := ""
	nodeSettings := []NodeSetting{}
	for _, nodeConfig := range config.NodeConfigs {
		decodedStakingSigningKey, err := base64.StdEncoding.DecodeString(nodeConfig.StakingSigningKey)
		if err != nil {
			return err
		}
		httpPort, err := utils.GetJSONKey[float64](nodeConfig.Flags, avagoconfig.HTTPPortKey)
		if err != nil {
			return fmt.Errorf("failure reading legacy local network conf: %w", err)
		}
		stakingPort, err := utils.GetJSONKey[float64](nodeConfig.Flags, avagoconfig.StakingPortKey)
		if err != nil {
			return fmt.Errorf("failure reading legacy local network conf: %w", err)
		}
		trackSubnetsStr, err = utils.GetJSONKey[string](nodeConfig.Flags, avagoconfig.TrackSubnetsKey)
		if err != nil && !errors.Is(err, constants.ErrKeyNotFoundOnMap) {
			return fmt.Errorf("failure reading legacy local network conf: %w", err)
		}
		nodeSettings = append(nodeSettings, NodeSetting{
			StakingTLSKey:    []byte(nodeConfig.StakingKey),
			StakingCertKey:   []byte(nodeConfig.StakingCert),
			StakingSignerKey: decodedStakingSigningKey,
			HTTPPort:         uint64(httpPort),
			StakingPort:      uint64(stakingPort),
		})
	}
	var trackedSubnets []ids.ID
	trackSubnetsStr = strings.TrimSpace(trackSubnetsStr)
	if trackSubnetsStr != "" {
		trackedSubnets, err = utils.MapWithError(strings.Split(trackSubnetsStr, ","), ids.FromString)
		if err != nil {
			return err
		}
	}
	binPath := config.BinaryPath
	// local connection info
	networkModel := models.NetworkFromNetworkID(connectionSettings.NetworkID)
	if networkModel.Kind == models.Local {
		genesisPath := filepath.Join(anrDir, "node1", "configs", "genesis.json")
		if !utils.FileExists(genesisPath) {
			return fmt.Errorf("genesis path not found at %s for local network cluster", genesisPath)
		}
		connectionSettings.Genesis, err = os.ReadFile(genesisPath)
		if err != nil {
			return err
		}
		upgradePath := filepath.Join(anrDir, "node1", "configs", "upgrade.json")
		if !utils.FileExists(upgradePath) {
			return fmt.Errorf("upgrade path not found at %s for local network cluster", upgradePath)
		}
		connectionSettings.Upgrade, err = os.ReadFile(upgradePath)
		if err != nil {
			return err
		}
		for nodeID, nodeIP := range config.BeaconConfig {
			connectionSettings.BootstrapIDs = append(connectionSettings.BootstrapIDs, nodeID.String())
			connectionSettings.BootstrapIPs = append(connectionSettings.BootstrapIPs, nodeIP.String())
		}
	}
	//
	pluginDir := filepath.Join(networkDir, "plugins")
	if err := os.MkdirAll(networkDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create network directory %s: %w", networkDir, err)
	}
	if err := os.MkdirAll(pluginDir, constants.DefaultPerms755); err != nil {
		return fmt.Errorf("could not create plugin directory %s: %w", pluginDir, err)
	}
	// defaultFlags
	defaultFlags := map[string]interface{}{}
	defaultFlags[avagoconfig.PartialSyncPrimaryNetworkKey] = true
	defaultFlags[avagoconfig.NetworkAllowPrivateIPsKey] = true
	defaultFlags[avagoconfig.IndexEnabledKey] = false
	defaultFlags[avagoconfig.IndexAllowIncompleteKey] = true
	network, err := CreateLocalCluster(
		app,
		printFunc,
		clusterName,
		binPath,
		nil,
		pluginDir,
		defaultFlags,
		connectionSettings,
		uint32(len(nodeSettings)),
		nodeSettings,
		trackedSubnets,
		networkModel,
		false,
		false,
	)
	if err != nil {
		return err
	}
	for i, node := range network.Nodes {
		sourceDir := filepath.Join(anrDir, config.NodeConfigs[i].Name, "db")
		targetDir := filepath.Join(networkDir, node.NodeID.String(), "db")
		if sdkutils.DirExists(sourceDir) {
			if err := dircopy.Copy(sourceDir, targetDir); err != nil {
				return fmt.Errorf("failure migrating data dir %s into %s: %w", sourceDir, targetDir, err)
			}
		}
		sourceDir = filepath.Join(anrDir, config.NodeConfigs[i].Name, "chainData")
		targetDir = filepath.Join(networkDir, node.NodeID.String(), "chainData")
		if sdkutils.DirExists(sourceDir) {
			if err := dircopy.Copy(sourceDir, targetDir); err != nil {
				return fmt.Errorf("failure migrating data dir %s into %s: %w", sourceDir, targetDir, err)
			}
		}
		sourceDir = filepath.Join(anrDir, config.NodeConfigs[i].Name, "plugins")
		targetDir = filepath.Join(networkDir, "plugins")
		if sdkutils.DirExists(sourceDir) {
			if err := dircopy.Copy(sourceDir, targetDir); err != nil {
				return fmt.Errorf("failure migrating plugindir dir %s into %s: %w", sourceDir, targetDir, err)
			}
		}
		sourceDir = filepath.Join(anrDir, config.NodeConfigs[i].Name, "configs", "chains")
		targetDir = filepath.Join(networkDir, node.NodeID.String(), "configs", "chains")
		if sdkutils.DirExists(sourceDir) {
			if err := dircopy.Copy(sourceDir, targetDir); err != nil {
				return fmt.Errorf("failure migrating chain configs dir %s into %s: %w", sourceDir, targetDir, err)
			}
		}
	}
	return os.RemoveAll(anrDir)
}

// isOldTmpNetVersion checks if the tmpnet directory uses the old configuration format
func isOldTmpNetVersion(networkDir string) (bool, error) {
	configPath := filepath.Join(networkDir, "config.json")
	if !utils.FileExists(configPath) {
		return false, nil // If no config exists, assume it's not old
	}

	// Read the network config file
	configData, err := utils.ReadJSON(configPath)
	if err != nil {
		return false, fmt.Errorf("failed to read config.json: %w", err)
	}

	// Check for old DefaultRuntimeConfig structure (indicates old version)
	if _, hasOldRuntime := configData["DefaultRuntimeConfig"]; hasOldRuntime {
		return true, nil
	}

	return false, nil // No old version indicators found
}

// migrateTmpNetToNewFormat migrates old tmpnet configuration format to new format
func migrateTmpNetToNewFormat(networkDir string) error {
	// Migrate network-level config.json
	configPath := filepath.Join(networkDir, "config.json")
	if utils.FileExists(configPath) {
		if err := migrateNetworkConfig(configPath); err != nil {
			return err
		}
	}

	// Migrate individual node config.json files and flags from flags.json
	entries, err := os.ReadDir(networkDir)
	if err != nil {
		return fmt.Errorf("failed to read network directory %s: %w", networkDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		nodeDir := filepath.Join(networkDir, entry.Name())
		if err := migrateNodeConfig(nodeDir); err != nil {
			return fmt.Errorf("failed to migrate node config in %s: %w", nodeDir, err)
		}
	}

	return nil
}

// migrateNetworkConfig migrates the network-level config.json from old to new format
func migrateNetworkConfig(configPath string) error {
	// Read the config file
	configData, err := utils.ReadJSON(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config.json: %w", err)
	}

	modified := false

	// Migrate DefaultFlags (convert mixed types to strings)
	if err := migrateNetworkDefaultFlags(configData, &modified); err != nil {
		return err
	}

	// Migrate DefaultRuntimeConfig structure
	if err := migrateNetworkDefaultRuntimeConfig(configData, &modified); err != nil {
		return err
	}

	// Migrate NetworkID
	if err := migrateNetworkID(configData, filepath.Dir(configPath), &modified); err != nil {
		return err
	}

	// Write back to file only if we made changes
	if modified {
		if err := utils.WriteJSON(configPath, configData); err != nil {
			return fmt.Errorf("failed to write fixed config.json: %w", err)
		}
	}

	return nil
}

// migrateNetworkDefaultFlags migrates DefaultFlags by converting mixed types to strings
func migrateNetworkDefaultFlags(configData map[string]interface{}, modified *bool) error {
	// Check if DefaultFlags exists and is a map
	defaultFlagsInterface, exists := configData["DefaultFlags"]
	if !exists {
		return nil // Nothing to fix if DefaultFlags doesn't exist
	}

	defaultFlagsMap, ok := defaultFlagsInterface.(map[string]interface{})
	if !ok {
		return nil // Nothing to fix if DefaultFlags is not a map
	}

	// Convert all values to strings
	for key, value := range defaultFlagsMap {
		if _, isString := value.(string); !isString {
			// Convert non-string values to strings
			defaultFlagsMap[key] = fmt.Sprintf("%v", value)
			*modified = true
		}
	}

	return nil
}

// migrateNetworkDefaultRuntimeConfig migrates DefaultRuntimeConfig to new structure
func migrateNetworkDefaultRuntimeConfig(configData map[string]interface{}, modified *bool) error {
	// Create the new default runtime config structure
	newDefaultRuntimeConfig := map[string]interface{}{
		"process": map[string]interface{}{},
	}
	processConfig := newDefaultRuntimeConfig["process"].(map[string]interface{})
	hasRuntimeData := false

	// Migrate from old DefaultRuntimeConfig structure
	if defaultRuntimeConfigInterface, oldExists := configData["DefaultRuntimeConfig"]; oldExists {
		if oldDefaultRuntimeConfig, ok := defaultRuntimeConfigInterface.(map[string]interface{}); ok {
			// Migrate AvalancheGoPath
			if avalancheGoPath, exists := oldDefaultRuntimeConfig["AvalancheGoPath"]; exists {
				processConfig["avalancheGoPath"] = avalancheGoPath
				hasRuntimeData = true
			}

			// Migrate ReuseDynamicPorts
			if reuseDynamicPorts, exists := oldDefaultRuntimeConfig["ReuseDynamicPorts"]; exists {
				processConfig["reuseDynamicPorts"] = reuseDynamicPorts
				hasRuntimeData = true
			}

			// Migrate PluginDir if it exists
			if pluginDir, exists := oldDefaultRuntimeConfig["PluginDir"]; exists {
				processConfig["pluginDir"] = pluginDir
				hasRuntimeData = true
			}

			// Remove the old structure
			delete(configData, "DefaultRuntimeConfig")
			*modified = true
		}
	}

	// Extract plugin-dir from DefaultFlags and move it to ProcessRuntimeConfig
	if defaultFlagsInterface, flagsExist := configData["DefaultFlags"]; flagsExist {
		if defaultFlagsMap, ok := defaultFlagsInterface.(map[string]interface{}); ok {
			if pluginDir, exists := defaultFlagsMap["plugin-dir"]; exists {
				processConfig["pluginDir"] = pluginDir
				// Remove plugin-dir from flags as it belongs in ProcessRuntimeConfig
				delete(defaultFlagsMap, "plugin-dir")
				// Update the DefaultFlags in configData after modification
				configData["DefaultFlags"] = defaultFlagsMap
				hasRuntimeData = true
				*modified = true
			}
		}
	}

	// Only set the default runtime config if we have actual data, otherwise set to nil
	if hasRuntimeData {
		configData["defaultRuntimeConfig"] = newDefaultRuntimeConfig
	} else {
		configData["defaultRuntimeConfig"] = nil
	}
	*modified = true

	return nil
}

// migrateNetworkID checks the network ID using old logic and sets NetworkID
func migrateNetworkID(configData map[string]interface{}, networkDir string, modified *bool) error {
	// Skip if NetworkID is already set
	if _, exists := configData["networkID"]; exists {
		return nil
	}

	// Try to get network ID using old logic
	networkID, err := getOldTmpNetNetworkID(networkDir)
	if err != nil {
		// If we can't determine the network ID, don't set it
		return nil
	}

	// Always set NetworkID
	configData["networkID"] = networkID
	*modified = true

	return nil
}

// getOldTmpNetNetworkID implements the old network ID retrieval logic for migration purposes
func getOldTmpNetNetworkID(networkDir string) (uint32, error) {
	// Find the first node directory
	entries, err := os.ReadDir(networkDir)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Try to read the node's flags
		flagsPath := filepath.Join(networkDir, entry.Name(), "flags.json")
		if !utils.FileExists(flagsPath) {
			continue
		}

		flagsData, err := utils.ReadJSON(flagsPath)
		if err != nil {
			continue
		}

		// Get network-name flag (this was config.NetworkNameKey in the old logic)
		if networkNameInterface, exists := flagsData[avagoconfig.NetworkNameKey]; exists {
			if networkNameStr, ok := networkNameInterface.(string); ok {
				networkID, err := strconv.ParseUint(networkNameStr, 10, 32)
				if err == nil {
					return uint32(networkID), nil
				}
			}
		}
	}

	return 0, fmt.Errorf("could not determine network ID from old format")
}

// migrateNodeConfig migrates individual node config.json from old to new format
func migrateNodeConfig(nodeDir string) error {
	configPath := filepath.Join(nodeDir, "config.json")
	flagsPath := filepath.Join(nodeDir, "flags.json")

	if !utils.FileExists(configPath) {
		return nil // Skip if no node config exists
	}

	// Read the node config
	configData, err := utils.ReadJSON(configPath)
	if err != nil {
		return fmt.Errorf("failed to read node config.json: %w", err)
	}

	modified := false

	// Migrate flags section
	if err := migrateNodeFlags(configData, flagsPath, &modified); err != nil {
		return err
	}

	// Migrate runtime config structure (this should be done after flags are loaded)
	if err := migrateNodeRuntimeConfig(configData, &modified); err != nil {
		return err
	}

	// Migrate blockchain configurations from files to content
	if err := migrateNodeBlockchainConfigs(configData, &modified); err != nil {
		return err
	}

	// Update the config if we made changes
	if modified {
		if err := utils.WriteJSON(configPath, configData); err != nil {
			return fmt.Errorf("failed to write fixed node config.json: %w", err)
		}
	}

	return nil
}

// migrateNodeFlags migrates node flags from flags.json to config.json and converts types
func migrateNodeFlags(configData map[string]interface{}, flagsPath string, modified *bool) error {
	// Check if flags section exists and is empty/missing
	flagsInterface, flagsExist := configData["flags"]
	var flagsMap map[string]interface{}
	var ok bool

	if flagsExist {
		flagsMap, ok = flagsInterface.(map[string]interface{})
		if !ok {
			flagsMap = make(map[string]interface{})
		}
	} else {
		flagsMap = make(map[string]interface{})
	}

	// If flags are empty/missing and flags.json exists, migrate the flags
	if len(flagsMap) == 0 && utils.FileExists(flagsPath) {
		flagsData, err := utils.ReadJSON(flagsPath)
		if err != nil {
			return fmt.Errorf("failed to read flags.json: %w", err)
		}

		// Copy all flags from flags.json to the config flags section
		for key, value := range flagsData {
			flagsMap[key] = value
		}
		*modified = true
	}

	// Convert all flag values to strings (fix mixed types)
	for key, value := range flagsMap {
		if _, isString := value.(string); !isString {
			flagsMap[key] = fmt.Sprintf("%v", value)
			*modified = true
		}
	}

	// Update the config with the fixed flags
	if *modified {
		configData["flags"] = flagsMap
	}

	return nil
}

// migrateNodeRuntimeConfig migrates node runtime config to new tmpnet format
func migrateNodeRuntimeConfig(configData map[string]interface{}, modified *bool) error {
	// Create the new runtime config structure
	newRuntimeConfig := map[string]interface{}{
		"process": map[string]interface{}{},
	}
	processConfig := newRuntimeConfig["process"].(map[string]interface{})
	hasRuntimeData := false

	// Migrate from old RuntimeConfig structure
	if runtimeConfigInterface, oldExists := configData["RuntimeConfig"]; oldExists {
		if oldRuntimeConfig, ok := runtimeConfigInterface.(map[string]interface{}); ok {
			// Migrate AvalancheGoPath
			if avalancheGoPath, exists := oldRuntimeConfig["AvalancheGoPath"]; exists {
				processConfig["avalancheGoPath"] = avalancheGoPath
				hasRuntimeData = true
			}

			// Migrate ReuseDynamicPorts
			if reuseDynamicPorts, exists := oldRuntimeConfig["ReuseDynamicPorts"]; exists {
				processConfig["reuseDynamicPorts"] = reuseDynamicPorts
				hasRuntimeData = true
			}

			// Migrate PluginDir if it exists
			if pluginDir, exists := oldRuntimeConfig["PluginDir"]; exists {
				processConfig["pluginDir"] = pluginDir
				hasRuntimeData = true
			}

			// Remove the old structure
			delete(configData, "RuntimeConfig")
			*modified = true
		}
	}

	// Extract plugin-dir from flags and move it to ProcessRuntimeConfig
	if flagsInterface, flagsExist := configData["flags"]; flagsExist {
		if flagsMap, ok := flagsInterface.(map[string]interface{}); ok {
			if pluginDir, exists := flagsMap["plugin-dir"]; exists {
				processConfig["pluginDir"] = pluginDir
				// Remove plugin-dir from flags as it belongs in ProcessRuntimeConfig
				delete(flagsMap, "plugin-dir")
				// Update the flags in configData after modification
				configData["flags"] = flagsMap
				hasRuntimeData = true
				*modified = true
			}
		}
	}

	// Only set the runtime config if we have actual data, otherwise set to nil
	if hasRuntimeData {
		configData["runtimeConfig"] = newRuntimeConfig
	} else {
		configData["runtimeConfig"] = nil
	}
	*modified = true

	return nil
}

// migrateNodeBlockchainConfigs migrates blockchain configurations from files to ChainConfigContentKey
func migrateNodeBlockchainConfigs(configData map[string]interface{}, modified *bool) error {
	// Check if flags section exists
	flagsInterface, flagsExist := configData["flags"]
	if !flagsExist {
		return nil // No flags section to work with
	}

	flagsMap, ok := flagsInterface.(map[string]interface{})
	if !ok {
		return nil // Flags is not a map
	}

	// Look for chain configuration files in the old structure
	chainsDirI, hasDir := flagsMap[avagoconfig.ChainConfigDirKey]
	if !hasDir {
		return nil // No chain config directory configured
	}
	chainsDir, ok := chainsDirI.(string)
	if !ok {
		return fmt.Errorf("unexpected type for %s flag, expected string, found %I", avagoconfig.ChainConfigDirKey, chainsDirI)
	}

	// Read all blockchain configurations from the chains directory
	blockchainConfigs := make(map[string]chains.ChainConfig)

	entries, err := os.ReadDir(chainsDir)
	if err != nil {
		return fmt.Errorf("failed to read chains directory %s: %w", chainsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		blockchainID := entry.Name()
		blockchainDir := filepath.Join(chainsDir, blockchainID)

		// Read config.json if it exists
		configPath := filepath.Join(blockchainDir, "config.json")
		var configData []byte
		if utils.FileExists(configPath) {
			configData, err = os.ReadFile(configPath)
			if err != nil {
				return fmt.Errorf("failed to read blockchain config %s: %w", configPath, err)
			}
		}

		// Read upgrade.json if it exists
		upgradePath := filepath.Join(blockchainDir, "upgrade.json")
		var upgradeData []byte
		if utils.FileExists(upgradePath) {
			upgradeData, err = os.ReadFile(upgradePath)
			if err != nil {
				return fmt.Errorf("failed to read blockchain upgrade %s: %w", upgradePath, err)
			}
		}

		blockchainConfig := chains.ChainConfig{}
		if len(configData) > 0 {
			blockchainConfig.Config = configData
		}
		if len(upgradeData) > 0 {
			blockchainConfig.Upgrade = upgradeData
		}
		// Only add to configs if we have at least one file
		if len(configData) > 0 || len(upgradeData) > 0 {
			blockchainConfigs[blockchainID] = blockchainConfig
		}
	}

	// If we found any blockchain configurations, migrate them
	if len(blockchainConfigs) > 0 {
		// Marshal the blockchain configurations
		marshaledBlockchainConfigs, err := json.Marshal(blockchainConfigs)
		if err != nil {
			return fmt.Errorf("failed to marshal blockchain configs: %w", err)
		}

		// Encode as base64 and set in ChainConfigContentKey
		flagsMap[avagoconfig.ChainConfigContentKey] = base64.StdEncoding.EncodeToString(marshaledBlockchainConfigs)

		// Remove the old ChainConfigDirKey
		delete(flagsMap, avagoconfig.ChainConfigDirKey)

		// Update the config data
		configData["flags"] = flagsMap
		*modified = true
	}

	return nil
}
