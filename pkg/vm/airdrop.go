package vm

import (
	"errors"
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/subnet-evm/core"
)

const stageAfterAirdrop = precompileStage
const stageBeforeAirdrop = feeStage

func getDefaultAllocation() (core.GenesisAlloc, error) {
	allocation := core.GenesisAlloc{}
	defaultAmount, ok := new(big.Int).SetString(defaultAirdropAmount, 10)
	if !ok {
		return allocation, errors.New("Unable to decode default allocation")
	}

	allocation[Prefunded_ewoq_Address] = core.GenesisAccount{
		Balance: defaultAmount,
	}
	return allocation, nil
}

func getAllocation() (core.GenesisAlloc, creationStage, error) {
	allocation := core.GenesisAlloc{}

	defaultAirdrop := "Airdrop 1 million tokens to the default address (do not use in production)"
	customAirdrop := "Customize your airdrop"
	extendAirdrop := "Would you like to airdrop more tokens?"

	airdropType, err := prompts.CaptureList(
		"How would you like to distribute funds",
		[]string{defaultAirdrop, customAirdrop, goBackMsg},
	)
	if err != nil {
		return allocation, errored, err
	}

	if airdropType == defaultAirdrop {
		alloc, err := getDefaultAllocation()
		return alloc, stageAfterAirdrop, err
	}

	if airdropType == goBackMsg {
		return allocation, stageBeforeAirdrop, nil
	}

	for {
		addressHex, err := prompts.CaptureAddress("Address to airdrop to")
		if err != nil {
			return nil, errored, err
		}

		amount, err := prompts.CapturePositiveBigInt("Amount to airdrop (in AVAX units)")
		if err != nil {
			return nil, errored, err
		}

		amount = amount.Mul(amount, oneAvax)

		account := core.GenesisAccount{
			Balance: amount,
		}

		allocation[addressHex] = account

		continueAirdrop, err := prompts.CaptureNoYes(extendAirdrop)
		if err != nil {
			return nil, errored, err
		}
		if !continueAirdrop {
			return allocation, stageAfterAirdrop, nil
		}
	}
}
