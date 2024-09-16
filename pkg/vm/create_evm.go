// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
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
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/core"
	subnetevmparams "github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/utils"
	"github.com/ethereum/go-ethereum/common"
)

var (
	// 600 AVAX: to deploy teleporter contract, registry contract, and fund
	// starting relayer operations
	teleporterBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(600))
	// 1000 AVAX: to deploy teleporter contract, registry contract, fund
	// starting relayer operations, and deploy bridge contracts
	externalGasTokenBalance = big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(1000))
)

func CreateEvmSidecar(
	app *application.Avalanche,
	subnetName string,
	subnetEVMVersion string,
	tokenSymbol string,
	getRPCVersionFromBinary bool,
) (*models.Sidecar, error) {
	var (
		err        error
		rpcVersion int
	)

	if getRPCVersionFromBinary {
		_, vmBin, err := binutils.SetupSubnetEVM(app, subnetEVMVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to install subnet-evm: %w", err)
		}
		rpcVersion, err = GetVMBinaryProtocolVersion(vmBin)
		if err != nil {
			return nil, fmt.Errorf("unable to get RPC version: %w", err)
		}
	} else {
		rpcVersion, err = GetRPCProtocolVersion(app, models.SubnetEvm, subnetEVMVersion)
		if err != nil {
			return nil, err
		}
	}

	sc := models.Sidecar{
		Name:        subnetName,
		VM:          models.SubnetEvm,
		VMVersion:   subnetEVMVersion,
		RPCVersion:  rpcVersion,
		Subnet:      subnetName,
		TokenSymbol: tokenSymbol,
		TokenName:   tokenSymbol + " Token",
	}

	return &sc, nil
}

func CreateEVMGenesis(
	app *application.Avalanche,
	blockchainName string,
	params SubnetEVMGenesisParams,
	teleporterInfo *teleporter.Info,
) ([]byte, error) {
	ux.Logger.PrintToUser("creating genesis for blockchain %s", blockchainName)

	genesis := core.Genesis{}
	genesis.Timestamp = *utils.TimeToNewUint64(time.Now())
	conf := subnetevmparams.SubnetEVMDefaultChainConfig
	conf.NetworkUpgrades = subnetevmparams.NetworkUpgrades{}

	chainID := new(big.Int).SetUint64(params.chainID)
	conf.ChainID = chainID

	setFeeConfig(params, conf)

	// Validity checks on the parameter settings.
	if params.enableTransactionPrecompile {
		if someoneWasAllowed(params.transactionPrecompileAllowList) &&
			!someAllowedHasBalance(params.transactionPrecompileAllowList, params.initialTokenAllocation) {
			return nil, errors.New("none of the addresses in the transaction allow list precompile have any tokens allocated to them. Currently, no address can transact on the network. Allocate some funds to one of the allow list addresses to continue")
		}
	}
	if (params.UseTeleporter || params.UseExternalGasToken) && !params.enableWarpPrecompile {
		return nil, fmt.Errorf("a teleporter enabled blockchain was requested but warp precompile is disabled")
	}
	if (params.UseTeleporter || params.UseExternalGasToken) && teleporterInfo == nil {
		return nil, fmt.Errorf("a teleporter enabled blockchain was requested but no teleporter info was provided")
	}

	// Add the teleporter deployer to the initial token allocation if necessary.
	if params.UseTeleporter || params.UseExternalGasToken {
		balance := teleporterBalance
		if params.UseExternalGasToken {
			balance = externalGasTokenBalance
		}
		params.initialTokenAllocation[common.HexToAddress(teleporterInfo.FundedAddress)] = core.GenesisAccount{
			Balance: balance,
		}
		addICMMessengerContractToGenesisAllocations(params.initialTokenAllocation)
		if err := addICMRegistryContractToGenesisAllocations(params.initialTokenAllocation); err != nil {
			return nil, err
		}
	}

	if params.UseExternalGasToken {
		params.enableNativeMinterPrecompile = true
		params.nativeMinterPrecompileAllowList.AdminAddresses = append(
			params.nativeMinterPrecompileAllowList.AdminAddresses,
			common.HexToAddress(teleporterInfo.FundedAddress),
		)
	}

	getPrecompiles(conf, params, &genesis.Timestamp)

	if params.UseTeleporter || params.UseExternalGasToken {
		addTeleporterAddressesToAllowLists(
			conf,
			teleporterInfo.FundedAddress,
			teleporterInfo.MessengerDeployerAddress,
			teleporterInfo.RelayerAddress,
		)
	}

	genesis.Alloc = params.initialTokenAllocation
	genesis.Config = conf
	genesis.Difficulty = Difficulty
	genesis.GasLimit = conf.FeeConfig.GasLimit.Uint64()

	jsonBytes, err := genesis.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, jsonBytes, "", "    ")
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
