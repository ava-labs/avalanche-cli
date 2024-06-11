// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridge

import (
	_ "embed"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/evm"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/liyue201/erc20-go/erc20"
)

type TeleporterFeeInfo struct {
	FeeTokenAddress common.Address
	Amount          *big.Int
}

func RegisterERC20Spoke(
	srcDir string,
	rpcURL string,
	prefundedPrivateKey string,
	address common.Address,
) error {
	srcDir = utils.ExpandHome(srcDir)
	abiPath := filepath.Join(srcDir, "contracts/out/ERC20TokenSpoke.sol/ERC20TokenSpoke.abi.json")
	abiBytes, err := os.ReadFile(abiPath)
	if err != nil {
		return err
	}
	metadata := &bind.MetaData{
		ABI: string(abiBytes),
	}
	abi, err := metadata.GetAbi()
	if err != nil {
		return err
	}
	client, err := evm.GetClient(rpcURL)
	if err != nil {
		return err
	}
	defer client.Close()
	txOpts, err := evm.GetTxOptsWithSigner(client, prefundedPrivateKey)
	if err != nil {
		return err
	}
	feeInfo := TeleporterFeeInfo{
		Amount: big.NewInt(0),
	}
	contract := bind.NewBoundContract(address, *abi, client, client, client)
	tx, err := contract.Transact(txOpts, "registerWithHub", feeInfo)
	if err != nil {
		return err
	}
	if _, success, err := evm.WaitForTransaction(client, tx); err != nil {
		return err
	} else if !success {
		return fmt.Errorf("failed receipt status deploying contract")
	}
	return nil
}

type TokenSpokeSettings struct {
	TeleporterRegistryAddress common.Address
	TeleporterManager         common.Address
	TokenHubBlockchainID      [32]byte
	TokenHubAddress           common.Address
	TokenHubDecimals          uint8
}

func DeployERC20Spoke(
	srcDir string,
	rpcURL string,
	prefundedPrivateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	tokenHubBlockchainID [32]byte,
	tokenHubAddress common.Address,
	tokenName string,
	tokenSymbol string,
	tokenDecimals uint8,
) (common.Address, error) {
	srcDir = utils.ExpandHome(srcDir)
	abiPath := filepath.Join(srcDir, "contracts/out/ERC20TokenSpoke.sol/ERC20TokenSpoke.abi.json")
	binPath := filepath.Join(srcDir, "contracts/out/ERC20TokenSpoke.sol/ERC20TokenSpoke.bin")
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
	tokenSpokeSettings := TokenSpokeSettings{
		TeleporterRegistryAddress: teleporterRegistryAddress,
		TeleporterManager:         teleporterManagerAddress,
		TokenHubBlockchainID:      tokenHubBlockchainID,
		TokenHubAddress:           tokenHubAddress,
		// TODO: user case for hub having diff decimals
		TokenHubDecimals: tokenDecimals,
	}
	address, tx, _, err := bind.DeployContract(
		txOpts,
		*abi,
		bin,
		client,
		tokenSpokeSettings,
		tokenName,
		tokenSymbol,
		tokenDecimals,
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

func DeployERC20Hub(
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

func DeployNativeHub(
	srcDir string,
	rpcURL string,
	prefundedPrivateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	wrappedNativeTokenAddress common.Address,
) (common.Address, error) {
	srcDir = utils.ExpandHome(srcDir)
	abiPath := filepath.Join(srcDir, "contracts/out/NativeTokenHub.sol/NativeTokenHub.abi.json")
	binPath := filepath.Join(srcDir, "contracts/out/NativeTokenHub.sol/NativeTokenHub.bin")
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
		wrappedNativeTokenAddress,
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

func DeployWrappedNativeToken(
	srcDir string,
	rpcURL string,
	prefundedPrivateKey string,
	tokenSymbol string,
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
	address, tx, _, err := bind.DeployContract(txOpts, *abi, bin, client, tokenSymbol)
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

func GetTokenParams(endpoint string, tokenAddress string) (string, string, uint8, error) {
	address := common.HexToAddress(tokenAddress)
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return "", "", 0, err
	}
	token, err := erc20.NewGGToken(address, client)
	if err != nil {
		return "", "", 0, err
	}
	tokenName, err := token.Name(nil)
	if err != nil {
		return "", "", 0, err
	}
	tokenSymbol, err := token.Symbol(nil)
	if err != nil {
		return "", "", 0, err
	}
	// TODO: find out if there are decimals options and why (academy)
	tokenDecimals, err := token.Decimals(nil)
	if err != nil {
		return "", "", 0, err
	}
	return tokenSymbol, tokenName, tokenDecimals, nil
}
