// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchain

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func PromptValidatorBalance(app *application.Avalanche, availableBalance uint64) (uint64, error) {
	ux.Logger.PrintToUser("Validator's balance is used to pay for continuous fee to the P-Chain")
	ux.Logger.PrintToUser("When this Balance reaches 0, the validator will be considered inactive and will no longer participate in validating the L1")
	txt := "What balance would you like to assign to the validator (in AVAX)?"
	return app.Prompt.CaptureValidatorBalance(txt, availableBalance, constants.BootstrapValidatorBalanceAVAX)
}

func GetKeyForChangeOwner(app *application.Avalanche, network models.Network) (string, error) {
	changeAddrPrompt := "Which key would you like to set as change owner for leftover AVAX if the node is removed from validator set?"

	const (
		getFromStored = "Get address from an existing stored key (created from avalanche key create or avalanche key import)"
		custom        = "Custom"
	)

	listOptions := []string{getFromStored, custom}
	listDecision, err := app.Prompt.CaptureList(changeAddrPrompt, listOptions)
	if err != nil {
		return "", err
	}

	var key string

	switch listDecision {
	case getFromStored:
		key, err = prompts.CaptureKeyAddress(
			app.Prompt,
			"be set as a change owner for leftover AVAX",
			app.GetKeyDir(),
			app.GetKey,
			network,
			prompts.PChainFormat,
		)
		if err != nil {
			return "", err
		}
	case custom:
		addrPrompt := "Enter change address (P-chain format)"
		changeAddr, err := app.Prompt.CaptureAddress(addrPrompt)
		if err != nil {
			return "", err
		}
		key = changeAddr.String()
	}
	if err != nil {
		return "", err
	}
	return key, nil
}
