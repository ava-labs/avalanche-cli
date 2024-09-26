// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package contract

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/subnet-evm/precompile/contracts/nativeminter"
	"github.com/ethereum/go-ethereum/common"
)

// returns information for the blockchain default allocation key
// if found, returns
// key name, address, private key
func GetDefaultBlockchainAirdropKeyInfo(
	app *application.Avalanche,
	blockchainName string,
) (string, string, string, error) {
	keyName := utils.GetDefaultBlockchainAirdropKeyName(blockchainName)
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
// 1) default CLI allocation key for blockchains
// 2) ewoq
// 3) all other stored keys managed by CLI
// returns address + private key when found
func GetBlockchainAirdropKeyInfo(
	app *application.Avalanche,
	network models.Network,
	blockchainName string,
	genesisData []byte,
) (string, string, string, error) {
	genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisData)
	if err != nil {
		return "", "", "", err
	}
	if blockchainName != "" {
		airdropKeyName, airdropAddress, airdropPrivKey, err := GetDefaultBlockchainAirdropKeyInfo(app, blockchainName)
		if err != nil {
			return "", "", "", err
		}
		for address := range genesis.Alloc {
			if address.Hex() == airdropAddress {
				return airdropKeyName, airdropAddress, airdropPrivKey, nil
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
		found, keyName, addressStr, privKey, err := searchForManagedKey(app, network, address, false)
		if err != nil {
			return "", "", "", err
		}
		if found {
			return keyName, addressStr, privKey, nil
		}
	}
	return "", "", "", nil
}

func searchForManagedKey(
	app *application.Avalanche,
	network models.Network,
	address common.Address,
	includeEwoq bool,
) (bool, string, string, string, error) {
	keyNames, err := utils.GetKeyNames(app.GetKeyDir(), includeEwoq)
	if err != nil {
		return false, "", "", "", err
	}
	for _, keyName := range keyNames {
		if k, err := app.GetKey(keyName, network, false); err != nil {
			return false, "", "", "", err
		} else if address.Hex() == k.C() {
			return true, keyName, k.C(), k.PrivKeyHex(), nil
		}
	}
	return false, "", "", "", nil
}

// get the deployed blockchain genesis, and then look for known
// private keys inside it
// returns address + private key when found
func GetEVMSubnetPrefundedKey(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (string, string, error) {
	genesisData, err := GetBlockchainGenesis(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return "", "", err
	}
	if !utils.ByteSliceIsSubnetEvmGenesis(genesisData) {
		return "", "", fmt.Errorf("search for prefunded key is only supported on EVM based vms")
	}
	_, genesisAddress, genesisPrivateKey, err := GetBlockchainAirdropKeyInfo(
		app,
		network,
		chainSpec.BlockchainName,
		genesisData,
	)
	if err != nil {
		return "", "", err
	}
	return genesisAddress, genesisPrivateKey, nil
}

// get the deployed blockchain genesis
func GetBlockchainGenesis(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) ([]byte, error) {
	blockchainID, err := GetBlockchainID(app, network, chainSpec)
	if err != nil {
		return nil, err
	}
	createChainTx, err := utils.GetBlockchainTx(network.Endpoint, blockchainID)
	if err != nil {
		return nil, err
	}
	return createChainTx.GenesisData, err
}

func sumGenesisSupply(
	genesisData []byte,
) (*big.Int, error) {
	sum := new(big.Int)
	genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisData)
	if err != nil {
		return sum, err
	}
	for _, allocation := range genesis.Alloc {
		sum.Add(sum, allocation.Balance)
	}
	return sum, nil
}

func GetEVMSubnetGenesisSupply(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (*big.Int, error) {
	genesisData, err := GetBlockchainGenesis(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return nil, err
	}
	if !utils.ByteSliceIsSubnetEvmGenesis(genesisData) {
		return nil, fmt.Errorf("genesis supply calculation is only supported on EVM based vms")
	}
	return sumGenesisSupply(genesisData)
}

func getGenesisNativeMinterAdmin(
	app *application.Avalanche,
	network models.Network,
	genesisData []byte,
) (bool, bool, string, string, string, error) {
	genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisData)
	if err != nil {
		return false, false, "", "", "", err
	}
	if genesis.Config != nil && genesis.Config.GenesisPrecompiles[nativeminter.ConfigKey] != nil {
		allowListCfg, ok := genesis.Config.GenesisPrecompiles[nativeminter.ConfigKey].(*nativeminter.Config)
		if !ok {
			return false, false, "", "", "", fmt.Errorf(
				"expected config of type nativeminter.AllowListConfig, but got %T",
				allowListCfg,
			)
		}
		if len(allowListCfg.AllowListConfig.AdminAddresses) == 0 {
			return false, false, "", "", "", nil
		}
		for _, admin := range allowListCfg.AllowListConfig.AdminAddresses {
			found, keyName, addressStr, privKey, err := searchForManagedKey(app, network, admin, true)
			if err != nil {
				return false, false, "", "", "", err
			}
			if found {
				return true, true, keyName, addressStr, privKey, nil
			}
		}
		return true, false, "", allowListCfg.AllowListConfig.AdminAddresses[0].Hex(), "", nil
	}
	return false, false, "", "", "", nil
}

func getGenesisNativeMinterManager(
	app *application.Avalanche,
	network models.Network,
	genesisData []byte,
) (bool, bool, string, string, string, error) {
	genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisData)
	if err != nil {
		return false, false, "", "", "", err
	}
	if genesis.Config != nil && genesis.Config.GenesisPrecompiles[nativeminter.ConfigKey] != nil {
		allowListCfg, ok := genesis.Config.GenesisPrecompiles[nativeminter.ConfigKey].(*nativeminter.Config)
		if !ok {
			return false, false, "", "", "", fmt.Errorf(
				"expected config of type nativeminter.AllowListConfig, but got %T",
				allowListCfg,
			)
		}
		if len(allowListCfg.AllowListConfig.ManagerAddresses) == 0 {
			return false, false, "", "", "", nil
		}
		for _, admin := range allowListCfg.AllowListConfig.ManagerAddresses {
			found, keyName, addressStr, privKey, err := searchForManagedKey(app, network, admin, true)
			if err != nil {
				return false, false, "", "", "", err
			}
			if found {
				return true, true, keyName, addressStr, privKey, nil
			}
		}
		return true, false, "", allowListCfg.AllowListConfig.ManagerAddresses[0].Hex(), "", nil
	}
	return false, false, "", "", "", nil
}

func GetEVMSubnetGenesisNativeMinterAdmin(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (bool, bool, string, string, string, error) {
	genesisData, err := GetBlockchainGenesis(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return false, false, "", "", "", err
	}
	if !utils.ByteSliceIsSubnetEvmGenesis(genesisData) {
		return false, false, "", "", "", fmt.Errorf("genesis native minter admin query is only supported on EVM based vms")
	}
	return getGenesisNativeMinterAdmin(app, network, genesisData)
}

func GetEVMSubnetGenesisNativeMinterManager(
	app *application.Avalanche,
	network models.Network,
	chainSpec ChainSpec,
) (bool, bool, string, string, string, error) {
	genesisData, err := GetBlockchainGenesis(
		app,
		network,
		chainSpec,
	)
	if err != nil {
		return false, false, "", "", "", err
	}
	if !utils.ByteSliceIsSubnetEvmGenesis(genesisData) {
		return false, false, "", "", "", fmt.Errorf("genesis native minter manager query is only supported on EVM based vms")
	}
	return getGenesisNativeMinterManager(app, network, genesisData)
}

func ContractAddressIsInGenesisData(
	genesisData []byte,
	contractAddress common.Address,
) (bool, error) {
	genesis, err := utils.ByteSliceToSubnetEvmGenesis(genesisData)
	if err != nil {
		return false, err
	}
	for address, allocation := range genesis.Alloc {
		if address == contractAddress {
			return len(allocation.Code) > 0, nil
		}
	}
	return false, nil
}
