package vm

import (
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/ux"
)

func getChainId() (*big.Int, error) {
	// TODO check against known chain ids and provide warning
	ux.Logger.PrintToUser("Enter your subnet's ChainId. It can be any positive integer.")

	chainId, err := prompts.CapturePositiveBigInt("ChainId")
	if err != nil {
		return nil, err
	}

	return chainId, nil
}

func getTokenName() (string, error) {
	ux.Logger.PrintToUser("Select a symbol for your subnet's native token")
	tokenName, err := prompts.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}

	return tokenName, nil
}

func getDescriptors() (*big.Int, string, stateDirection, error) {
	chainId, err := getChainId()
	if err != nil {
		return nil, "", kill, err
	}

	tokenName, err := getTokenName()
	if err != nil {
		return nil, "", kill, err
	}
	return chainId, tokenName, forward, nil
}
