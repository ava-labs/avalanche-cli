// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package vm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/interchain"
	icmgenesis "github.com/ava-labs/avalanche-cli/pkg/interchain/genesis"
	"github.com/ava-labs/avalanche-cli/pkg/interchain/relayer"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/validatormanager"
	blockchainSDK "github.com/ava-labs/avalanche-cli/sdk/blockchain"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ava-labs/subnet-evm/utils"
	"github.com/ethereum/go-ethereum/common"
)

var (
	// 600 AVAX: to deploy ICM contract, registry contract, and fund
	// starting relayer operations
	icmBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(600))
	// 1000 AVAX: to deploy ICM contract, registry contract, fund
	// starting relayer operations, and deploy bridge contracts
	externalGasTokenBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1000))
)

// Variables for testing - can be monkey patched
var (
	setupSubnetEVM        = binutils.SetupSubnetEVM
	getRPCProtocolVersion = GetRPCProtocolVersion
)

func CreateEvmSidecar(
	sc *models.Sidecar,
	app *application.Avalanche,
	subnetName string,
	subnetEVMVersion string,
	tokenSymbol string,
	getRPCVersionFromBinary bool,
	sovereign bool,
	useV2_0_0 bool,
) (*models.Sidecar, error) {
	var (
		err        error
		rpcVersion int
	)

	if sc == nil {
		sc = &models.Sidecar{}
	}

	if getRPCVersionFromBinary {
		_, vmBin, err := setupSubnetEVM(app, subnetEVMVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to install subnet-evm: %w", err)
		}
		rpcVersion, err = getVMBinaryProtocolVersion(vmBin)
		if err != nil {
			return nil, fmt.Errorf("unable to get RPC version: %w", err)
		}
	} else {
		rpcVersion, err = getRPCProtocolVersion(app, models.VMType(models.SubnetEvm), subnetEVMVersion)
		if err != nil {
			return nil, err
		}
	}

	sc.Name = subnetName
	sc.VM = models.VMType(models.SubnetEvm)
	sc.VMVersion = subnetEVMVersion
	sc.RPCVersion = rpcVersion
	sc.Subnet = subnetName
	sc.TokenSymbol = tokenSymbol
	sc.TokenName = tokenSymbol + " Token"
	sc.Sovereign = sovereign
	sc.UseACP99 = useV2_0_0
	return sc, nil
}

func CreateEVMGenesis(
	app *application.Avalanche,
	params SubnetEVMGenesisParams,
	icmInfo *interchain.ICMInfo,
	addICMRegistryToGenesis bool,
	proxyOwner string,
	rewardBasisPoints uint64,
	useV2_0_0 bool,
) ([]byte, error) {
	feeConfig := getFeeConfig(params)

	// Validity checks on the parameter settings.
	if params.enableTransactionPrecompile {
		if someoneWasAllowed(params.transactionPrecompileAllowList) &&
			!someAllowedHasBalance(params.transactionPrecompileAllowList, params.initialTokenAllocation) {
			return nil, errors.New("none of the addresses in the transaction allow list precompile have any tokens allocated to them. Currently, no address can transact on the network. Allocate some funds to one of the allow list addresses to continue")
		}
	}
	if (params.UseICM || params.UseExternalGasToken) && !params.enableWarpPrecompile {
		return nil, fmt.Errorf("a ICM enabled blockchain was requested but warp precompile is disabled")
	}
	if (params.UseICM || params.UseExternalGasToken) && icmInfo == nil {
		return nil, fmt.Errorf("a ICM enabled blockchain was requested but no ICM info was provided")
	}

	// Add the ICM deployer to the initial token allocation if necessary.
	if params.UseICM || params.UseExternalGasToken {
		balance := icmBalance
		if params.UseExternalGasToken {
			balance = externalGasTokenBalance
		}
		if params.initialTokenAllocation == nil {
			params.initialTokenAllocation = core.GenesisAlloc{}
		}
		params.initialTokenAllocation[common.HexToAddress(icmInfo.FundedAddress)] = core.GenesisAccount{
			Balance: balance,
		}
		if !params.DisableICMOnGenesis {
			icmgenesis.AddICMMessengerContractToAllocations(params.initialTokenAllocation)
			if addICMRegistryToGenesis {
				// experimental
				if err := icmgenesis.AddICMRegistryContractToAllocations(params.initialTokenAllocation); err != nil {
					return nil, err
				}
			}
		}
	}

	if params.UsePoAValidatorManager && params.UsePoSValidatorManager {
		return nil, fmt.Errorf("blockchain can not be both PoA and PoS")
	}
	if params.UsePoAValidatorManager {
		validatormanager.AddValidatorTransparentProxyContractToAllocations(params.initialTokenAllocation, proxyOwner)
		// valid for both v2.0.0 and v1.0.0
		validatormanager.AddValidatorMessagesV2_0_0ContractToAllocations(params.initialTokenAllocation)
		if useV2_0_0 {
			validatormanager.AddValidatorManagerV2_0_0ContractToAllocations(params.initialTokenAllocation)
		} else {
			validatormanager.AddPoAValidatorManagerV1_0_0ContractToAllocations(params.initialTokenAllocation)
		}
	} else if params.UsePoSValidatorManager {
		validatormanager.AddValidatorTransparentProxyContractToAllocations(params.initialTokenAllocation, proxyOwner)
		// valid for both v2.0.0 and v1.0.0
		validatormanager.AddValidatorMessagesV2_0_0ContractToAllocations(params.initialTokenAllocation)
		validatormanager.AddRewardCalculatorV2_0_0ToAllocations(params.initialTokenAllocation, rewardBasisPoints)
		if useV2_0_0 {
			validatormanager.AddSpecializationTransparentProxyContractToAllocations(params.initialTokenAllocation, proxyOwner)
		}
	}

	if params.UseExternalGasToken {
		params.enableNativeMinterPrecompile = true
		params.nativeMinterPrecompileAllowList.AdminAddresses = append(
			params.nativeMinterPrecompileAllowList.AdminAddresses,
			common.HexToAddress(icmInfo.FundedAddress),
		)
	}

	genesisBlock0Timestamp := utils.TimeToNewUint64(time.Now())
	precompiles := getPrecompiles(params, genesisBlock0Timestamp)

	_, relayerAddress, _, err := relayer.GetDefaultRelayerKeyInfo(app)
	if err != nil {
		return nil, err
	}
	if params.UseICM || params.UseExternalGasToken {
		addICMAddressesToAllowLists(
			&precompiles,
			icmInfo.FundedAddress,
			icmInfo.MessengerDeployerAddress,
			relayerAddress,
		)
	}

	subnetConfig, err := blockchainSDK.New(
		&blockchainSDK.SubnetParams{
			SubnetEVM: &blockchainSDK.SubnetEVMParams{
				ChainID:     new(big.Int).SetUint64(params.chainID),
				FeeConfig:   feeConfig,
				Allocation:  params.initialTokenAllocation,
				Precompiles: precompiles,
				Timestamp:   genesisBlock0Timestamp,
			},
			Name: "TestSubnet",
		})
	if err != nil {
		return nil, err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, subnetConfig.Genesis, "", "    ")
	if err != nil {
		return nil, err
	}

	return prettyJSON.Bytes(), nil
}

func someoneWasAllowed(allowList AllowList) bool {
	addrs := append(append(allowList.AdminAddresses, allowList.ManagerAddresses...), allowList.EnabledAddresses...)
	return len(addrs) > 0
}

func someAllowedHasBalance(allowList AllowList, allocations core.GenesisAlloc) bool {
	addrs := append(append(allowList.AdminAddresses, allowList.ManagerAddresses...), allowList.EnabledAddresses...)
	for _, addr := range addrs {
		// we can break at the first address that has a non-zero balance
		if bal, ok := allocations[addr]; ok &&
			bal.Balance != nil &&
			bal.Balance.Uint64() > uint64(0) {
			return true
		}
	}
	return false
}
