package vm

import (
	"errors"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/subnet-evm/core"
)

func getDefaultAllocation() (core.GenesisAlloc, error) {
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
	prompter prompts.PromptCreateFunc,
	selector prompts.SelectCreateFunc,
) (core.GenesisAlloc, stateDirection, error) {
	allocation := core.GenesisAlloc{}

	defaultAirdrop := "Airdrop 1 million tokens to the default address (do not use in production)"
	customAirdrop := "Customize your airdrop"
	extendAirdrop := "Would you like to airdrop more tokens?"

	airdropType, err := prompts.CaptureList(
		selector(
			"How would you like to distribute funds",
			[]string{defaultAirdrop, customAirdrop, goBackMsg},
		),
	)
	if err != nil {
		return allocation, stop, err
	}

	if airdropType == defaultAirdrop {
		alloc, err := getDefaultAllocation()
		return alloc, forward, err
	}

	if airdropType == goBackMsg {
		return allocation, backward, nil
	}

	for {
		addressHex, err := prompts.CaptureAddress(prompter("Address to airdrop to"))
		if err != nil {
			return nil, stop, err
		}

		amount, err := prompts.CapturePositiveBigInt(prompter("Amount to airdrop (in AVAX units)"))
		if err != nil {
			return nil, stop, err
		}

		amount = amount.Mul(amount, oneAvax)

		account := core.GenesisAccount{
			Balance: amount,
		}

		allocation[addressHex] = account

		continueAirdrop, err := prompts.CaptureNoYes(selector, extendAirdrop)
		if err != nil {
			return nil, stop, err
		}
		if !continueAirdrop {
			return allocation, forward, nil
		}
	}
}
