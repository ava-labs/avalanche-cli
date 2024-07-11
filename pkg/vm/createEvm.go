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
	"github.com/ava-labs/avalanche-cli/pkg/statemachine"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/core"
	subnetevmparams "github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/contracts/txallowlist"
	"github.com/ava-labs/subnet-evm/utils"
	"github.com/ethereum/go-ethereum/common"
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
			return &models.Sidecar{}, fmt.Errorf("failed to install subnet-evm: %w", err)
		}
		rpcVersion, err = GetVMBinaryProtocolVersion(vmBin)
		if err != nil {
			return &models.Sidecar{}, fmt.Errorf("unable to get RPC version: %w", err)
		}
	} else {
		rpcVersion, err = GetRPCProtocolVersion(app, models.SubnetEvm, subnetEVMVersion)
		if err != nil {
			return &models.Sidecar{}, err
		}
	}

	sc := &models.Sidecar{
		Name:        subnetName,
		VM:          models.SubnetEvm,
		VMVersion:   subnetEVMVersion,
		RPCVersion:  rpcVersion,
		Subnet:      subnetName,
		TokenSymbol: tokenSymbol,
		TokenName:   tokenSymbol + " Token",
	}

	return sc, nil
}

func CreateEvmGenesis(
	app *application.Avalanche,
	subnetName string,
	params SubnetEVMGenesisParams,
	subnetEVMVersion string,
	subnetEVMChainID uint64,
	tokenSymbol string,
	useSubnetEVMDefaults bool,
	useWarp bool,
	teleporterInfo *teleporter.Info,
) ([]byte, error) {
	ux.Logger.PrintToUser("creating genesis for subnet %s", subnetName)

	genesis := core.Genesis{}
	genesis.Timestamp = *utils.TimeToNewUint64(time.Now())
	conf := subnetevmparams.SubnetEVMDefaultChainConfig
	conf.NetworkUpgrades = subnetevmparams.NetworkUpgrades{}

	chainID := new(big.Int).SetUint64(params.chainID)
	conf.ChainID = chainID

	var (
		allocation core.GenesisAlloc
		direction  statemachine.StateDirection
		err        error
	)

	*conf, direction, err = GetFeeConfig(*conf, app, useSubnetEVMDefaults)

	allocation, direction, err = getAllocation(
		app,
		subnetName,
		defaultEvmAirdropAmount,
		oneAvax,
		fmt.Sprintf("Amount to airdrop (in %s units)", tokenSymbol),
		useSubnetEVMDefaults,
	)
	if teleporterInfo != nil {
		allocation = addTeleporterAddressToAllocations(
			allocation,
			teleporterInfo.FundedAddress,
			teleporterInfo.FundedBalance,
		)
	}
	*conf, direction, err = getPrecompiles(*conf, app, &genesis.Timestamp, useSubnetEVMDefaults, useWarp, subnetEVMVersion)
	if teleporterInfo != nil {
		*conf = addTeleporterAddressesToAllowLists(
			*conf,
			teleporterInfo.FundedAddress,
			teleporterInfo.MessengerDeployerAddress,
			teleporterInfo.RelayerAddress,
		)
	}

	if conf != nil && conf.GenesisPrecompiles[txallowlist.ConfigKey] != nil {
		allowListCfg, ok := conf.GenesisPrecompiles[txallowlist.ConfigKey].(*txallowlist.Config)
		if !ok {
			return nil, fmt.Errorf(
				"expected config of type txallowlist.AllowListConfig, but got %T",
				allowListCfg,
			)
		}

		if err := ensureAdminsHaveBalance(
			allowListCfg.AdminAddresses,
			allocation); err != nil {
			return nil, err
		}
	}

	genesis.Alloc = allocation
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

func ensureAdminsHaveBalance(admins []common.Address, alloc core.GenesisAlloc) error {
	if len(admins) < 1 {
		return nil
	}

	for _, admin := range admins {
		// we can break at the first admin who has a non-zero balance
		if bal, ok := alloc[admin]; ok &&
			bal.Balance != nil &&
			bal.Balance.Uint64() > uint64(0) {
			return nil
		}
	}
	return errors.New(
		"none of the addresses in the transaction allow list precompile have any tokens allocated to them. Currently, no address can transact on the network. Airdrop some funds to one of the allow list addresses to continue",
	)
}
