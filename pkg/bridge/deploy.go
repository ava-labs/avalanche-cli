// Cpopyright (C) 2022, Ava Labs, Inc. All rights reserved.
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

type TokenSpokeSettings struct {
	TeleporterRegistryAddress common.Address
	TeleporterManager         common.Address
	TokenHubBlockchainID      [32]byte
	TokenHubAddress           common.Address
	TokenHubDecimals          uint8
}

func RegisterERC20Spoke(
	rpcURL string,
	privateKey string,
	spokeAddress common.Address,
) error {
	feeInfo := TeleporterFeeInfo{
		Amount: big.NewInt(0),
	}
	return contract.TxToMethod(
		rpcURL,
		privateKey,
		spokeAddress,
		nil,
		"registerWithHub((address, uint256))",
		feeInfo,
	)
}

func DeployERC20Spoke(
	srcDir string,
	rpcURL string,
	privateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	tokenHubBlockchainID [32]byte,
	tokenHubAddress common.Address,
	tokenName string,
	tokenSymbol string,
	tokenDecimals uint8,
) (common.Address, error) {
	binPath := filepath.Join(srcDir, "contracts/out/ERC20TokenSpoke.sol/ERC20TokenSpoke.bin")
	binBytes, err := os.ReadFile(binPath)
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
	return contract.DeployContract(
		rpcURL,
		privateKey,
		binBytes,
		"((address, address, bytes32, address, uint8), string, string, uint8)",
		tokenSpokeSettings,
		tokenName,
		tokenSymbol,
		tokenDecimals,
	)
}

func DeployERC20Hub(
	srcDir string,
	rpcURL string,
	privateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	erc20TokenAddress common.Address,
	erc20TokenDecimals uint8,
) (common.Address, error) {
	binPath := filepath.Join(srcDir, "contracts/out/ERC20TokenHub.sol/ERC20TokenHub.bin")
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

func DeployNativeHub(
	srcDir string,
	rpcURL string,
	privateKey string,
	teleporterRegistryAddress common.Address,
	teleporterManagerAddress common.Address,
	wrappedNativeTokenAddress common.Address,
) (common.Address, error) {
	binPath := filepath.Join(srcDir, "contracts/out/NativeTokenHub.sol/NativeTokenHub.bin")
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
