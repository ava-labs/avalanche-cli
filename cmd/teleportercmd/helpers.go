// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package teleportercmd

import (
	"fmt"
	"strings"

	"github.com/ava-labs/avalanche-cli/cmd/subnetcmd"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/subnet"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanchego/ids"
)

type PrivateKeyFlags struct {
	PrivateKey string
	KeyName    string
	GenesisKey bool
}

func getPrivateKeyFromFlags(
	flags PrivateKeyFlags,
	genesisPrivateKey string,
) (string, error) {
	privateKey := flags.PrivateKey
	if flags.KeyName != "" {
		k, err := app.GetKey(flags.KeyName, models.NewLocalNetwork(), false)
		if err != nil {
			return "", err
		}
		privateKey = k.PrivKeyHex()
	}
	if flags.GenesisKey {
		privateKey = genesisPrivateKey
	}
	return privateKey, nil
}

func getEVMSubnetPrefundedKey(
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
			if b, _, err := subnetcmd.HasSubnetEVMGenesis(subnetName); err != nil {
				return "", "", err
			} else if !b {
				return "", "", fmt.Errorf("getPrefundedKey only works on EVM based vms")
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
		return "", "", fmt.Errorf("getPrefundedKey only works on EVM based vms")
	}
	_, genesisAddress, genesisPrivateKey, err := subnet.GetSubnetAirdropKeyInfo(
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

func promptPrivateKey(
	goal string,
	genesisAddress string,
	genesisPrivateKey string,
) (string, error) {
	privateKey := ""
	cliKeyOpt := "Get private key from an existing stored key (created from avalanche key create or avalanche key import)"
	customKeyOpt := "Custom"
	genesisKeyOpt := fmt.Sprintf("Use the private key of the Genesis Aidrop address %s", genesisAddress)
	keyOptions := []string{cliKeyOpt, customKeyOpt}
	if genesisPrivateKey != "" {
		keyOptions = []string{genesisKeyOpt, cliKeyOpt, customKeyOpt}
	}
	keyOption, err := app.Prompt.CaptureList(
		fmt.Sprintf("Which private key do you want to use to %s?", goal),
		keyOptions,
	)
	if err != nil {
		return "", err
	}
	switch keyOption {
	case cliKeyOpt:
		keyName, err := prompts.CaptureKeyName(app.Prompt, goal, app.GetKeyDir(), true)
		if err != nil {
			return "", err
		}
		k, err := app.GetKey(keyName, models.NewLocalNetwork(), false)
		if err != nil {
			return "", err
		}
		privateKey = k.PrivKeyHex()
	case customKeyOpt:
		privateKey, err = app.Prompt.CaptureString("Private Key")
		if err != nil {
			return "", err
		}
	case genesisKeyOpt:
		privateKey = genesisPrivateKey
	}
	return privateKey, nil
}

func isCChain(subnetName string) bool {
	return strings.ToLower(subnetName) == "c-chain" || strings.ToLower(subnetName) == "cchain"
}
