// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package ictt

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
	_, _, err := contract.TxToMethod(
		rpcURL,
		privateKey,
		remoteAddress,
		nil,
		"registerWithHome((address, uint256))",
		feeInfo,
	)
	return err
}

func DeployERC20Remote(
	srcDir string,
	rpcURL string,
	privateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	tokenHomeBlockchainID [32]byte,
	tokenHomeAddress common.Address,
	tokenHomeDecimals uint8,
	tokenRemoteName string,
	tokenRemoteSymbol string,
	tokenRemoteDecimals uint8,
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
		TokenHomeDecimals:         tokenHomeDecimals,
	}
	return contract.DeployContract(
		rpcURL,
		privateKey,
		binBytes,
		"((address, address, bytes32, address, uint8), string, string, uint8)",
		tokenRemoteSettings,
		tokenRemoteName,
		tokenRemoteSymbol,
		tokenRemoteDecimals,
	)
}

func DeployNativeRemote(
	srcDir string,
	rpcURL string,
	privateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	tokenHomeBlockchainID [32]byte,
	tokenHomeAddress common.Address,
	tokenHomeDecimals uint8,
	nativeAssetSymbol string,
	initialReserveImbalance *big.Int,
	burnedFeesReportingRewardPercentage *big.Int,
) (common.Address, error) {
	binPath := filepath.Join(srcDir, "contracts/out/NativeTokenRemote.sol/NativeTokenRemote.bin")
	binBytes, err := os.ReadFile(binPath)
	if err != nil {
		return common.Address{}, err
	}
	tokenRemoteSettings := TokenRemoteSettings{
		TeleporterRegistryAddress: teleporterRegistryAddress,
		TeleporterManager:         teleporterManagerAddress,
		TokenHomeBlockchainID:     tokenHomeBlockchainID,
		TokenHomeAddress:          tokenHomeAddress,
		TokenHomeDecimals:         tokenHomeDecimals,
	}
	return contract.DeployContract(
		rpcURL,
		privateKey,
		binBytes,
		"((address, address, bytes32, address, uint8), string, uint256, uint256)",
		tokenRemoteSettings,
		nativeAssetSymbol,
		initialReserveImbalance,
		burnedFeesReportingRewardPercentage,
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
