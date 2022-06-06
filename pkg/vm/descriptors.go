package vm

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
)

const stageAfterDescriptors = feeStage

func getChainId() (*big.Int, error) {
	// TODO check against known chain ids and provide warning
	fmt.Println("Enter your subnet's ChainId. It can be any positive integer.")

	chainId, err := prompts.CapturePositiveBigInt("ChainId")
	if err != nil {
		return nil, err
	}

	return chainId, nil
}

func getTokenName() (string, error) {
	fmt.Println("Select a symbol for your subnet's native token")
	tokenName, err := prompts.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}

	return tokenName, nil
}

func getDescriptors() (*big.Int, string, creationStage, error) {
	chainId, err := getChainId()
	if err != nil {
		return nil, "", errored, err
	}

	tokenName, err := getTokenName()
	if err != nil {
		return nil, "", errored, err
	}
	return chainId, tokenName, stageAfterDescriptors, nil
}
