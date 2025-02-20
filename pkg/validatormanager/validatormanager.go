// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package validatormanager

import (
	_ "embed"
	"fmt"
	"github.com/ava-labs/avalanche-cli/pkg/contract"
	"math/big"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/models"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	validatorManagerSDK "github.com/ava-labs/avalanche-cli/sdk/validatormanager"
	"github.com/ava-labs/avalanchego/api/info"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed deployed_validator_messages_bytecode_v1.0.0.txt
var deployedValidatorMessagesBytecode []byte

func AddValidatorMessagesContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedValidatorMessagesBytes := common.FromHex(strings.TrimSpace(string(deployedValidatorMessagesBytecode)))
	allocs[common.HexToAddress(validatorManagerSDK.ValidatorMessagesContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedValidatorMessagesBytes,
		Nonce:   1,
	}
}

func IsValidatorManagerPoA(
	rpcURL string,
	managerAddress common.Address,
) bool {
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"weightToValue(uint64)->(uint256)",
		uint64(1),
	)
	// if it is PoA it will return Error: execution reverted
	if err != nil {
		return true
	}
	_, ok := out[0].(*big.Int)
	return !ok
}

func GetValidatorManagerOwner(
	rpcURL string,
	managerAddress common.Address,
) (common.Address, error) {
	out, err := contract.CallToMethod(
		rpcURL,
		managerAddress,
		"owner()->(address)",
	)
	if err != nil {
		return common.Address{}, err
	}

	ownerAddr, ok := out[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("error at owner() call, expected common.Address, got %T", out[0])
	}
	return ownerAddr, nil
}

func fillValidatorMessagesAddressPlaceholder(contract string) string {
	return strings.ReplaceAll(
		contract,
		"__$fd0c147b4031eef6079b0498cbafa865f0$__",
		validatorManagerSDK.ValidatorMessagesContractAddress[2:],
	)
}

//go:embed deployed_poa_validator_manager_bytecode_v1.0.0.txt
var deployedPoAValidatorManagerBytecode []byte

func AddPoAValidatorManagerContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedPoaValidatorManagerString := strings.TrimSpace(string(deployedPoAValidatorManagerBytecode))
	deployedPoaValidatorManagerString = fillValidatorMessagesAddressPlaceholder(deployedPoaValidatorManagerString)
	deployedPoaValidatorManagerBytes := common.FromHex(deployedPoaValidatorManagerString)
	allocs[common.HexToAddress(validatorManagerSDK.ValidatorContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedPoaValidatorManagerBytes,
		Nonce:   1,
	}
}

//go:embed deployed_native_pos_validator_manager_bytecode.txt
var deployedPoSValidatorManagerBytecode []byte

func AddPoSValidatorManagerContractToAllocations(
	allocs core.GenesisAlloc,
) {
	deployedPoSValidatorManagerBytes := common.FromHex(strings.TrimSpace(string(deployedPoSValidatorManagerBytecode)))
	allocs[common.HexToAddress(validatorManagerSDK.ValidatorContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedPoSValidatorManagerBytes,
		Nonce:   1,
	}
}

//go:embed deployed_transparent_proxy_bytecode.txt
var deployedTransparentProxyBytecode []byte

//go:embed deployed_proxy_admin_bytecode.txt
var deployedProxyAdminBytecode []byte

func AddTransparentProxyContractToAllocations(
	allocs core.GenesisAlloc,
	proxyManager string,
) {
	// proxy admin
	deployedProxyAdmin := common.FromHex(strings.TrimSpace(string(deployedProxyAdminBytecode)))
	allocs[common.HexToAddress(validatorManagerSDK.ProxyAdminContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedProxyAdmin,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x0"): common.HexToHash(proxyManager),
		},
	}

	// transparent proxy
	deployedTransparentProxy := common.FromHex(strings.TrimSpace(string(deployedTransparentProxyBytecode)))
	allocs[common.HexToAddress(validatorManagerSDK.ProxyContractAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedTransparentProxy,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"): common.HexToHash(validatorManagerSDK.ValidatorContractAddress),  // sslot for address of ValidatorManager logic -> bytes32(uint256(keccak256('eip1967.proxy.implementation')) - 1)
			common.HexToHash("0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103"): common.HexToHash(validatorManagerSDK.ProxyAdminContractAddress), // sslot for address of ProxyAdmin -> bytes32(uint256(keccak256('eip1967.proxy.admin')) - 1)
			// we can omit 3rd sslot for _data, as we initialize ValidatorManager after chain is live
		},
	}
}

//go:embed deployed_reward_calculator_bytecode.txt
var deployedRewardCalculatorBytecode []byte

func AddRewardCalculatorToAllocations(
	allocs core.GenesisAlloc,
	rewardBasisPoints uint64,
) {
	deployedRewardCalculatorBytes := common.FromHex(strings.TrimSpace(string(deployedRewardCalculatorBytecode)))
	allocs[common.HexToAddress(validatorManagerSDK.RewardCalculatorAddress)] = core.GenesisAccount{
		Balance: big.NewInt(0),
		Code:    deployedRewardCalculatorBytes,
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			common.HexToHash("0x0"): common.BigToHash(new(big.Int).SetUint64(rewardBasisPoints)),
		},
	}
}

// setups PoA manager after a successful execution of
// ConvertSubnetToL1Tx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func SetupPoA(
	subnet blockchainSDK.Subnet,
	network models.Network,
	privateKey string,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	validatorManagerAddressStr string,
) error {
	return subnet.InitializeProofOfAuthority(
		network,
		privateKey,
		aggregatorExtraPeerEndpoints,
		aggregatorAllowPrivatePeers,
		aggregatorLogger,
		validatorManagerAddressStr,
	)
}

// setups PoA manager after a successful execution of
// ConvertSubnetToL1Tx on P-Chain
// needs the list of validators for that tx,
// [convertSubnetValidators], together with an evm [ownerAddress]
// to set as the owner of the PoA manager
func SetupPoS(
	subnet blockchainSDK.Subnet,
	network models.Network,
	privateKey string,
	aggregatorExtraPeerEndpoints []info.Peer,
	aggregatorAllowPrivatePeers bool,
	aggregatorLogger logging.Logger,
	posParams validatorManagerSDK.PoSParams,
	validatorManagerAddressStr string,
) error {
	return subnet.InitializeProofOfStake(network,
		privateKey,
		aggregatorExtraPeerEndpoints,
		aggregatorAllowPrivatePeers,
		aggregatorLogger,
		posParams,
		validatorManagerAddressStr,
	)
}
