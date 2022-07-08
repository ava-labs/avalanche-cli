package vm

import (
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func getChainID(prompter prompts.PromptCreateFunc, app *application.Avalanche) (*big.Int, error) {
	// TODO check against known chain ids and provide warning
	ux.Logger.PrintToUser("Enter your subnet's ChainId. It can be any positive integer.")

	chainID, err := prompts.CapturePositiveBigInt(prompter("ChainId"))
	if err != nil {
		return nil, err
	}

	exists, err := app.ChainIDExists(chainID.String())
	if err != nil {
		return nil, err
	}
	if exists {
		ux.Logger.PrintToUser("The provided chain ID %q already exists! Try a different one:", chainID.String())
		return getChainID(prompter, app)
	}

	return chainID, nil
}

func getTokenName(prompter prompts.PromptCreateFunc) (string, error) {
	ux.Logger.PrintToUser("Select a symbol for your subnet's native token")
	tokenName, err := prompts.CaptureString(prompter("Token symbol"))
	if err != nil {
		return "", err
	}

	return tokenName, nil
}

func getDescriptors(app *application.Avalanche, prompter prompts.PromptCreateFunc) (*big.Int, string, stateDirection, error) {
	chainID, err := getChainID(prompter, app)
	if err != nil {
		return nil, "", stop, err
	}

	tokenName, err := getTokenName(prompter)
	if err != nil {
		return nil, "", stop, err
	}
	return chainID, tokenName, forward, nil
}
