// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/statemachine"
	"github.com/ava-labs/subnet-evm/core"
	"github.com/ethereum/go-ethereum/common"
)

const (
	defaultAirdrop = "Airdrop 1 million tokens to the default address (do not use in production)"
	customAirdrop  = "Customize your airdrop"
	extendAirdrop  = "Would you like to airdrop more tokens?"
)

func getDefaultAllocation(defaultAirdropAmount string) (core.GenesisAlloc, error) {
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
	defaultAirdropAmount string,
	multiplier *big.Int,
	captureAmountLabel string,
	useDefaults bool,
) (core.GenesisAlloc, statemachine.StateDirection, error) {
	if useDefaults {
		alloc, err := getDefaultAllocation(defaultAirdropAmount)
		return alloc, statemachine.Forward, err
	}

	allocation := core.GenesisAlloc{}

	airdropType, err := app.Prompt.CaptureList(
		"How would you like to distribute funds",
		[]string{defaultAirdrop, customAirdrop, goBackMsg},
	)
	if err != nil {
		return allocation, statemachine.Stop, err
	}

	if airdropType == defaultAirdrop {
		alloc, err := getDefaultAllocation(defaultAirdropAmount)
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
