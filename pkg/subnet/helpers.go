// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
)

var errIllegalNameCharacter = errors.New(
	"illegal name character: only letters, no special characters allowed")

func GetDefaultSubnetAirdropKeyInfo(app *application.Avalanche, subnetName string) (string, string, string, error) {
	keyName := utils.GetDefaultBlockchainAirdropKeyName(subnetName)
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

func ValidateSubnetNameAndGetChains(app *application.Avalanche, args []string) ([]string, error) {
	// this should not be necessary but some bright guy might just be creating
	// the genesis by hand or something...
	if err := checkInvalidSubnetNames(args[0]); err != nil {
		return nil, fmt.Errorf("subnet name %s is invalid: %w", args[0], err)
	}
	// Check subnet exists
	// TODO create a file that lists chains by subnet for fast querying
	chains, err := getChainsInSubnet(app, args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to getChainsInSubnet: %w", err)
	}

	if len(chains) == 0 {
		return nil, errors.New("Invalid subnet " + args[0])
	}

	return chains, nil
}

func checkInvalidSubnetNames(name string) error {
	// this is currently exactly the same code as in avalanchego/vms/platformvm/create_chain_tx.go
	for _, r := range name {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ') {
			return errIllegalNameCharacter
		}
	}
	return nil
}

func getChainsInSubnet(app *application.Avalanche, blockchainName string) ([]string, error) {
	subnets, err := os.ReadDir(app.GetSubnetDir())
	if err != nil {
		return nil, fmt.Errorf("failed to read baseDir: %w", err)
	}

	chains := []string{}

	for _, s := range subnets {
		if !s.IsDir() {
			continue
		}
		sidecarFile := filepath.Join(app.GetSubnetDir(), s.Name(), constants.SidecarFileName)
		if _, err := os.Stat(sidecarFile); err == nil {
			// read in sidecar file
			jsonBytes, err := os.ReadFile(sidecarFile)
			if err != nil {
				return nil, fmt.Errorf("failed reading file %s: %w", sidecarFile, err)
			}

			var sc models.Sidecar
			err = json.Unmarshal(jsonBytes, &sc)
			if err != nil {
				return nil, fmt.Errorf("failed unmarshaling file %s: %w", sidecarFile, err)
			}
			if sc.Subnet == blockchainName {
				chains = append(chains, sc.Name)
			}
		}
	}
	return chains, nil
}
