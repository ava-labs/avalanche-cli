// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"fmt"
	"os"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
)

// Tracks the given [blockchainName] at network given on [networkDir]
// After P-Chain is bootstrapped, set alias [blockchainName]->blockchainID
// for the network, and persists RPC into sidecar
// Use both for local networks and local clusters
func TrackSubnet(
	app *application.Avalanche,
	printFunc func(msg string, args ...interface{}),
	blockchainName string,
	networkDir string,
	wallet *primary.Wallet,
) error {
	networkModel, err := GetNetworkModel(networkDir)
	if err != nil {
		return err
	}
	sc, err := app.LoadSidecar(blockchainName)
	if err != nil {
		return err
	}
	if sc.Networks[networkModel.Name()].BlockchainID == ids.Empty {
		return fmt.Errorf("blockchain %s has not been deployed to %s", blockchainName, networkModel.Name())
	}
	blockchainID := sc.Networks[networkModel.Name()].BlockchainID
	subnetID := sc.Networks[networkModel.Name()].SubnetID
	var (
		blockchainConfig []byte
		subnetConfig     []byte
	)
	vmBinaryPath, err := SetupVMBinary(app, blockchainName)
	if err != nil {
		return fmt.Errorf("failed to setup VM binary: %w", err)
	}
	if app.ChainConfigExists(blockchainName) {
		blockchainConfig, err = os.ReadFile(app.GetChainConfigPath(blockchainName))
		if err != nil {
			return err
		}
	}
	if app.AvagoSubnetConfigExists(blockchainName) {
		subnetConfig, err = os.ReadFile(app.GetAvagoSubnetConfigPath(blockchainName))
		if err != nil {
			return err
		}
	}
	perNodeBlockchainConfig, err := app.GetPerNodeBlockchainConfig(blockchainName)
	if err != nil {
		return err
	}
	ctx, cancel := networkModel.BootstrappingContext()
	defer cancel()
	if err := TmpNetTrackSubnet(
		ctx,
		app.Log,
		printFunc,
		networkDir,
		blockchainName,
		sc.Sovereign,
		blockchainID,
		subnetID,
		vmBinaryPath,
		blockchainConfig,
		subnetConfig,
		perNodeBlockchainConfig,
		wallet,
	); err != nil {
		return err
	}
	ux.Logger.GreenCheckmarkToUser("%s successfully tracking %s", networkModel.Name(), blockchainName)
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return err
	}
	if networkModel.Kind == models.Local {
		if err := TmpNetSetAlias(network.Nodes, blockchainID.String(), blockchainName, subnetID); err != nil {
			return err
		}
	}
	nodeURIs, err := GetTmpNetNodeURIsWithFix(networkDir)
	if err != nil {
		return err
	}
	_, err = app.AddDefaultBlockchainRPCsToSidecar(
		blockchainName,
		networkModel,
		nodeURIs,
	)
	return err
}

// Returns the network model for the network at [networkDir]
func GetNetworkModel(
	networkDir string,
) (models.Network, error) {
	network, err := GetTmpNetNetwork(networkDir)
	if err != nil {
		return models.UndefinedNetwork, err
	}
	networkID, err := GetTmpNetNetworkID(network)
	if err != nil {
		return models.UndefinedNetwork, err
	}
	return models.NetworkFromNetworkID(networkID), nil
}
