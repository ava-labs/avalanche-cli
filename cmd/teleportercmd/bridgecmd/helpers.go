// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridgecmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

func deployERC20Hub(
	srcDir string,
	rpcURL string,
	prefundedPrivateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	erc20TokenAddress common.Address,
	erc20TokenDecimals uint8,
) (common.Address, error) {
	srcDir = utils.ExpandHome(srcDir)
	abiPath := filepath.Join(srcDir, "contracts/out/ERC20TokenHub.sol/ERC20TokenHub.abi.json")
	binPath := filepath.Join(srcDir, "contracts/out/ERC20TokenHub.sol/ERC20TokenHub.bin")
	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return common.Address{}, err
	}
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return common.Address{}, err
	}
	metadata := &bind.MetaData{
		ABI: string(abiBytes),
		Bin: string(binBytes),
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return common.Address{}, err
	}
	bin := common.FromHex(metadata.Bin)
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return common.Address{}, err
	}
	defer client.Close()
	txOpts, err := evm.GetTxOptsWithSigner(client, prefundedPrivateKey)
	if err != nil {
		return common.Address{}, err
	}
	address, tx, _, err := bind.DeployContract(
		txOpts,
		*abi,
		bin,
		client,
		teleporterRegistryAddress,
		teleporterManagerAddress,
		erc20TokenAddress,
		erc20TokenDecimals,
	)
	if err != nil {
		return common.Address{}, err
	}
	if _, success, err := evm.WaitForTransaction(client, tx); err != nil {
		return common.Address{}, err
	} else if !success {
		return common.Address{}, fmt.Errorf("failed receipt status deploying contract")
	}
	return address, nil
}

func deployNativeHub(
	srcDir string,
	rpcURL string,
	prefundedPrivateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	wrappedNativeTokenAddress common.Address,
) (common.Address, error) {
	srcDir = utils.ExpandHome(srcDir)
	abiPath := filepath.Join(srcDir, "contracts/out/NativeTokenSource.sol/NativeTokenSource.abi.json")
	binPath := filepath.Join(srcDir, "contracts/out/NativeTokenSource.sol/NativeTokenSource.bin")
	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return common.Address{}, err
	}
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return common.Address{}, err
	}
	metadata := &bind.MetaData{
		ABI: string(abiBytes),
		Bin: string(binBytes),
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return common.Address{}, err
	}
	bin := common.FromHex(metadata.Bin)
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return common.Address{}, err
	}
	defer client.Close()
	txOpts, err := evm.GetTxOptsWithSigner(client, prefundedPrivateKey)
	if err != nil {
		return common.Address{}, err
	}
	address, tx, _, err := bind.DeployContract(txOpts, *abi, bin, client, teleporterRegistryAddress, teleporterManagerAddress, wrappedNativeTokenAddress)
	if err != nil {
		return common.Address{}, err
	}
	if _, success, err := evm.WaitForTransaction(client, tx); err != nil {
		return common.Address{}, err
	} else if !success {
		return common.Address{}, fmt.Errorf("failed receipt status deploying contract")
	}
	return address, nil
}

func deployWrappedNativeToken(
	srcDir string,
	rpcURL string,
	prefundedPrivateKey string,
	tokenName string,
) (common.Address, error) {
	srcDir = utils.ExpandHome(srcDir)
	abiPath := filepath.Join(srcDir, "contracts/out/WrappedNativeToken.sol/WrappedNativeToken.abi.json")
	binPath := filepath.Join(srcDir, "contracts/out/WrappedNativeToken.sol/WrappedNativeToken.bin")
	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return common.Address{}, err
	}
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return common.Address{}, err
	}
	metadata := &bind.MetaData{
		ABI: string(abiBytes),
		Bin: string(binBytes),
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return common.Address{}, err
	}
	bin := common.FromHex(metadata.Bin)
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return common.Address{}, err
	}
	defer client.Close()
	txOpts, err := evm.GetTxOptsWithSigner(client, prefundedPrivateKey)
	if err != nil {
		return common.Address{}, err
	}
	address, tx, _, err := bind.DeployContract(txOpts, *abi, bin, client, tokenName)
	if err != nil {
		return common.Address{}, err
	}
	if _, success, err := evm.WaitForTransaction(client, tx); err != nil {
		return common.Address{}, err
	} else if !success {
		return common.Address{}, fmt.Errorf("failed receipt status deploying contract")
	}
	return address, nil
}
