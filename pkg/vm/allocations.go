// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
)

func addAllocation(allocations core.GenesisAlloc, address string, amount *big.Int) {
	allocations[common.HexToAddress(address)] = core.GenesisAccount{
		Balance: amount,
	}
}

func getNewAllocation(
	app *application.Avalanche,
	subnetName string,
	defaultAirdropAmount string,
) (core.GenesisAlloc, error) {
	keyName := utils.GetDefaultSubnetAirdropKeyName(subnetName)
	k, err := app.GetKey(keyName, models.NewLocalNetwork(), true)
	if err != nil {
		return core.GenesisAlloc{}, err
	}
	ux.Logger.PrintToUser("prefunding address %s with balance %s", k.C(), defaultAirdropAmount)
	allocations := core.GenesisAlloc{}
	defaultAmount, ok := new(big.Int).SetString(defaultAirdropAmount, 10)
	if !ok {
		return allocations, errors.New("unable to decode default allocation")
	}
	addAllocation(allocations, k.C(), defaultAmount)
	return allocations, nil
}

func getEwoqAllocation(defaultAirdropAmount string) (core.GenesisAlloc, error) {
	allocations := core.GenesisAlloc{}
	defaultAmount, ok := new(big.Int).SetString(defaultAirdropAmount, 10)
	if !ok {
		return allocations, errors.New("unable to decode default allocation")
	}

	ux.Logger.PrintToUser("prefunding address %s with balance %s", PrefundedEwoqAddress, defaultAirdropAmount)
	addAllocation(allocations, PrefundedEwoqAddress.String(), defaultAmount)
	return allocations, nil
}

func addInterchainMessagingAllocation(
	allocations core.GenesisAlloc,
	teleporterKeyAddress string,
	teleporterKeyBalance *big.Int,
) core.GenesisAlloc {
	if allocations != nil {
		addAllocation(allocations, teleporterKeyAddress, teleporterKeyBalance)
	}
	return allocations
}

func getAllocation(
	params SubnetEVMGenesisParams,
	app *application.Avalanche,
	subnetName string,
	defaultAirdropAmount string,
	multiplier *big.Int,
) (core.GenesisAlloc, error) {
	if params.initialTokenAllocation.allocToNewKey {
		return getNewAllocation(app, subnetName, defaultAirdropAmount)
	}

	if params.initialTokenAllocation.allocToEwoq {
		return getEwoqAllocation(defaultAirdropAmount)
	}

	allocations := core.GenesisAlloc{}
	amount := new(big.Int).SetUint64(params.initialTokenAllocation.customBalance)
	amount = amount.Mul(amount, multiplier)
	addAllocation(allocations, params.initialTokenAllocation.customAddress.Hex(), amount)
	return allocations, nil
}
