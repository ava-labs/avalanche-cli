// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/statemachine"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
)

const (
	newAirdrop    = "Airdrop 1 million tokens to a newly generate address (stored key)"
	ewoqAirdrop   = "Airdrop 1 million tokens to the default ewoq address (do not use in production)"
	customAirdrop = "Customize your airdrop"
	extendAirdrop = "Would you like to airdrop more tokens?"
)

func GetSubnetAirdropKeyName(subnetName string) string {
	return "subnet_" + subnetName + "_airdrop"
}

func getNewAllocation(app *application.Avalanche, subnetName string, defaultAirdropAmount string) (core.GenesisAlloc, error) {
	keyName := GetSubnetAirdropKeyName(subnetName)
	keyPath := app.GetKeyPath(keyName)
	var (
		k   *key.SoftKey
		err error
	)
	if utils.FileExists(keyPath) {
		k, err = key.LoadSoft(models.NewLocalNetwork().ID, keyPath)
		if err != nil {
			return core.GenesisAlloc{}, err
		}
	} else {
		k, err = key.NewSoft(0)
		if err != nil {
			return core.GenesisAlloc{}, err
		}
		if err := k.Save(keyPath); err != nil {
			return core.GenesisAlloc{}, err
		}
	}
	ux.Logger.PrintToUser("configuring airdrop to stored key %q with address %s", keyName, k.C())
	allocation := core.GenesisAlloc{}
	defaultAmount, ok := new(big.Int).SetString(defaultAirdropAmount, 10)
	if !ok {
		return allocation, errors.New("unable to decode default allocation")
	}

	allocation[common.HexToAddress(k.C())] = core.GenesisAccount{
		Balance: defaultAmount,
	}
	return allocation, nil
}

func getEwoqAllocation(defaultAirdropAmount string) (core.GenesisAlloc, error) {
	allocation := core.GenesisAlloc{}
	defaultAmount, ok := new(big.Int).SetString(defaultAirdropAmount, 10)
	if !ok {
		return allocation, errors.New("unable to decode default allocation")
	}

	allocation[PrefundedEwoqAddress] = core.GenesisAccount{
		Balance: defaultAmount,
	}
	return allocation, nil
}

func getAllocation(
	app *application.Avalanche,
	subnetName string,
	defaultAirdropAmount string,
	multiplier *big.Int,
	captureAmountLabel string,
	useDefaults bool,
) (core.GenesisAlloc, statemachine.StateDirection, error) {
	if useDefaults {
		alloc, err := getNewAllocation(app, subnetName, defaultAirdropAmount)
		return alloc, statemachine.Forward, err
	}

	allocation := core.GenesisAlloc{}

	airdropType, err := app.Prompt.CaptureList(
		"How would you like to distribute funds",
		[]string{newAirdrop, ewoqAirdrop, customAirdrop, goBackMsg},
	)
	if err != nil {
		return allocation, statemachine.Stop, err
	}

	if airdropType == newAirdrop {
		alloc, err := getNewAllocation(app, subnetName, defaultAirdropAmount)
		return alloc, statemachine.Forward, err
	}

	if airdropType == ewoqAirdrop {
		alloc, err := getEwoqAllocation(defaultAirdropAmount)
		return alloc, statemachine.Forward, err
	}

	if airdropType == goBackMsg {
		return allocation, statemachine.Backward, nil
	}

	var addressHex common.Address

	for {
		addressHex, err = app.Prompt.CaptureAddress("Address to airdrop to")
		if err != nil {
			return nil, statemachine.Stop, err
		}

		amount, err := app.Prompt.CapturePositiveBigInt(captureAmountLabel)
		if err != nil {
			return nil, statemachine.Stop, err
		}

		amount = amount.Mul(amount, multiplier)

		account, ok := allocation[addressHex]
		if !ok {
			account.Balance = big.NewInt(0)
		}
		account.Balance.Add(account.Balance, amount)

		allocation[addressHex] = account

		continueAirdrop, err := app.Prompt.CaptureNoYes(extendAirdrop)
		if err != nil {
			return nil, statemachine.Stop, err
		}
		if !continueAirdrop {
			return allocation, statemachine.Forward, nil
		}
	}
}
