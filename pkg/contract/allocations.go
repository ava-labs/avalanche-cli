// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"
)

// returns information for the subnet default allocation key
// if found, returns
// key name, address, private key
func GetDefaultSubnetAirdropKeyInfo(
	app *application.Avalanche,
	subnetName string,
) (string, string, string, error) {
	keyName := utils.GetDefaultSubnetAirdropKeyName(subnetName)
	keyPath := app.GetKeyPath(keyName)
	if utils.FileExists(keyPath) {
		k, err := key.LoadSoft(models.NewLocalNetwork().ID, keyPath)
		if err != nil {
			return "", "", "", err
		}
		return keyName, k.C(), k.PrivKeyHex(), nil
	}
	return "", "", "", nil
}

// from a given genesis, look for known private keys inside it, giving
// preference to the ones expected to be default
// it searches for:
// 1) default CLI allocation key for subnets
// 2) ewoq
// 3) all other stored keys managed by CLI
// returns address + private key when found
func GetSubnetAirdropKeyInfo(
	app *application.Avalanche,
	network models.Network,
	subnetName string,
	genesisData []byte,
) (string, string, string, error) {
	genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisData)
	if err != nil {
		return "", "", "", err
	}
	if subnetName != "" {
		subnetAirdropKeyName, subnetAirdropAddress, subnetAirdropPrivKey, err := GetDefaultSubnetAirdropKeyInfo(app, subnetName)
		if err != nil {
			return "", "", "", err
		}
		for address := range genesis.Alloc {
			if address.Hex() == subnetAirdropAddress {
				return subnetAirdropKeyName, subnetAirdropAddress, subnetAirdropPrivKey, nil
			}
		}
	}
	ewoq, err := app.GetKey("ewoq", network, false)
	if err != nil {
		return "", "", "", err
	}
	for address := range genesis.Alloc {
		if address.Hex() == ewoq.C() {
			return "ewoq", ewoq.C(), ewoq.PrivKeyHex(), nil
		}
	}
	for address := range genesis.Alloc {
		keyNames, err := utils.GetKeyNames(app.GetKeyDir(), false)
		if err != nil {
			return "", "", "", err
		}
		for _, keyName := range keyNames {
			if k, err := app.GetKey(keyName, network, false); err != nil {
				return "", "", "", err
			} else if address.Hex() == k.C() {
				return keyName, k.C(), k.PrivKeyHex(), nil
			}
		}
	}
	return "", "", "", nil
}

// get the deployed subnet genesis, and then look for known
// private keys inside it
// returns address + private key when found
func GetEVMSubnetPrefundedKey(
	app *application.Avalanche,
	network models.Network,
	subnetName string,
	isCChain bool,
	blockchainID string,
) (string, string, error) {
	if blockchainID == "" {
		if isCChain {
			blockchainID = "C"
		} else {
			sc, err := app.LoadSidecar(subnetName)
			if err != nil {
				return "", "", fmt.Errorf("failed to load sidecar: %w", err)
			}
			if b, _, err := app.HasSubnetEVMGenesis(subnetName); err != nil {
				return "", "", err
			} else if !b {
				return "", "", fmt.Errorf("search for prefunded key is only supported on EVM based vms")
			}
			if sc.Networks[network.Name()].BlockchainID == ids.Empty {
				return "", "", fmt.Errorf("subnet has not been deployed to %s", network.Name())
			}
			blockchainID = sc.Networks[network.Name()].BlockchainID.String()
		}
	}
	var (
		err     error
		chainID ids.ID
	)
	if isCChain || !network.StandardPublicEndpoint() {
		chainID, err = utils.GetChainID(network.Endpoint, blockchainID)
		if err != nil {
			return "", "", err
		}
	} else {
		chainID, err = ids.FromString(blockchainID)
		if err != nil {
			return "", "", err
		}
	}
	createChainTx, err := utils.GetBlockchainTx(network.Endpoint, chainID)
	if err != nil {
		return "", "", err
	}
	if !utils.ByteSliceIsSubnetEvmGenesis(createChainTx.GenesisData) {
		return "", "", fmt.Errorf("search for prefunded key is only supported on EVM based vms")
	}
	_, genesisAddress, genesisPrivateKey, err := GetSubnetAirdropKeyInfo(
		app,
		network,
		subnetName,
		createChainTx.GenesisData,
	)
	if err != nil {
		return "", "", err
	}
	return genesisAddress, genesisPrivateKey, nil
}
