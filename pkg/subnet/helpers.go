// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

func GetDefaultSubnetAirdropKeyInfo(app *application.Avalanche, subnetName string) (string, string, string, error) {
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

func GetSubnetAirdropKeyInfo(app *application.Avalanche, network models.Network, subnetName string, genesisData []byte) (string, string, string, error) {
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
