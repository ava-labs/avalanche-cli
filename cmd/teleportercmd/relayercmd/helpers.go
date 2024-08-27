// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package relayercmd

import (
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func AddBlockchainToClusterConf(network models.Network, cloudNodeID string, blockchainName string) error {
	relayerAddress, relayerPrivateKey, err := teleporter.GetRelayerKeyInfo(app.GetKeyPath(constants.AWMRelayerKeyName))
	if err != nil {
		return err
	}

	_, _, subnetID, chainID, messengerAddress, registryAddress, _, err := teleporter.GetBlockchainParams(app, network, "", true)
	if err != nil {
		return err
	}

	storageBasePath := constants.AWMRelayerDockerDir
	configBasePath := app.GetNodeInstanceDirPath(cloudNodeID)

	configPath := app.GetAWMRelayerServiceConfigPath(configBasePath)
	if err := os.MkdirAll(filepath.Dir(configPath), constants.DefaultPerms755); err != nil {
		return err
	}
	ux.Logger.PrintToUser("updating configuration file %s", configPath)

	if err = teleporter.UpdateRelayerConfig(
		configPath,
		app.GetAWMRelayerServiceStorageDir(storageBasePath),
		relayerAddress,
		relayerPrivateKey,
		network,
		subnetID.String(),
		chainID.String(),
		messengerAddress,
		registryAddress,
	); err != nil {
		return err
	}

	_, _, subnetID, chainID, messengerAddress, registryAddress, _, err = teleporter.GetBlockchainParams(app, network, blockchainName, false)
	if err != nil {
		return err
	}

	if err = teleporter.UpdateRelayerConfig(
		configPath,
		app.GetAWMRelayerServiceStorageDir(storageBasePath),
		relayerAddress,
		relayerPrivateKey,
		network,
		subnetID.String(),
		chainID.String(),
		messengerAddress,
		registryAddress,
	); err != nil {
		return err
	}

	return nil
}
