package vm

import (
	"math/big"

	"github.com/ava-labs/avalanche-cli/cmd/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/app"
	"github.com/ava-labs/avalanche-cli/ux"
)

func getChainId(app *app.Avalanche) (*big.Int, error) {
	// TODO check against known chain ids and provide warning
	ux.Logger.PrintToUser("Enter your subnet's ChainId. It can be any positive integer.")

	chainID, err := prompts.CapturePositiveBigInt("ChainId")
	if err != nil {
		return nil, err
	}

	exists, err := app.ChainIDExists(chainID.String())
	if err != nil {
		return nil, err
	}
	if exists {
		ux.Logger.PrintToUser("The provided chain ID %q already exists! Try a different one:", chainID.String())
		return getChainId(app)
	}

	return chainID, nil
}

func getTokenName() (string, error) {
	ux.Logger.PrintToUser("Select a symbol for your subnet's native token")
	tokenName, err := prompts.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}

	return tokenName, nil
}

func getDescriptors(app *app.Avalanche) (*big.Int, string, stateDirection, error) {
	chainId, err := getChainId(app)
	if err != nil {
		return nil, "", stop, err
	}

	tokenName, err := getTokenName()
	if err != nil {
		return nil, "", stop, err
	}
	return chainId, tokenName, forward, nil
}
