// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package bridge

import (
	_ "embed"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ethereum/go-ethereum/common"
)

type TeleporterFeeInfo struct {
	FeeTokenAddress common.Address
	Amount          *big.Int
}

type TokenRemoteSettings struct {
	TeleporterRegistryAddress common.Address
	TeleporterManager         common.Address
	TokenHomeBlockchainID     [32]byte
	TokenHomeAddress          common.Address
	TokenHomeDecimals         uint8
}

func RegisterERC20Remote(
	rpcURL string,
	privateKey string,
	remoteAddress common.Address,
) error {
	feeInfo := TeleporterFeeInfo{
		Amount: big.NewInt(0),
	}
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		remoteAddress,
		nil,
		"registerWithHome((address, uint256))",
		feeInfo,
	)
}

func DeployERC20Remote(
	srcDir string,
	rpcURL string,
	privateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	tokenHomeBlockchainID [32]byte,
	tokenHomeAddress common.Address,
	tokenName string,
	tokenSymbol string,
	tokenDecimals uint8,
) (common.Address, error) {
	binPath := filepath.Join(srcDir, "contracts/out/ERC20TokenRemote.sol/ERC20TokenRemote.bin")
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return common.Address{}, err
	}
	tokenRemoteSettings := TokenRemoteSettings{
		TeleporterRegistryAddress: teleporterRegistryAddress,
		TeleporterManager:         teleporterManagerAddress,
		TokenHomeBlockchainID:     tokenHomeBlockchainID,
		TokenHomeAddress:          tokenHomeAddress,
		// TODO: user case for home having diff decimals
		TokenHomeDecimals: tokenDecimals,
	}
	return contract.DeployContract(
		rpcURL,
		privateKey,
		binBytes,
		"((address, address, bytes32, address, uint8), string, string, uint8)",
		tokenRemoteSettings,
		tokenName,
		tokenSymbol,
		tokenDecimals,
	)
}

func DeployERC20Home(
	srcDir string,
	rpcURL string,
	privateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	erc20TokenAddress common.Address,
	erc20TokenDecimals uint8,
) (common.Address, error) {
	binPath := filepath.Join(srcDir, "contracts/out/ERC20TokenHome.sol/ERC20TokenHome.bin")
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return common.Address{}, err
	}
	return contract.DeployContract(
		rpcURL,
		privateKey,
		binBytes,
		"(address, address, address, uint8)",
		teleporterRegistryAddress,
		teleporterManagerAddress,
		erc20TokenAddress,
		erc20TokenDecimals,
	)
}

func DeployNativeHome(
	srcDir string,
	rpcURL string,
	privateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	wrappedNativeTokenAddress common.Address,
) (common.Address, error) {
	binPath := filepath.Join(srcDir, "contracts/out/NativeTokenHome.sol/NativeTokenHome.bin")
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return common.Address{}, err
	}
	return contract.DeployContract(
		rpcURL,
		privateKey,
		binBytes,
		"(address, address, address)",
		teleporterRegistryAddress,
		teleporterManagerAddress,
		wrappedNativeTokenAddress,
	)
}

func DeployWrappedNativeToken(
	srcDir string,
	rpcURL string,
	privateKey string,
	tokenSymbol string,
) (common.Address, error) {
	binPath := filepath.Join(utils.ExpandHome(srcDir), "contracts/out/WrappedNativeToken.sol/WrappedNativeToken.bin")
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return common.Address{}, err
	}
	return contract.DeployContract(
		rpcURL,
		privateKey,
		binBytes,
		"(string)",
		tokenSymbol,
	)
}
