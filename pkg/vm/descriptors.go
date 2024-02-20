// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/statemachine"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func getChainID(app *application.Avalanche, subnetEVMChainID uint64) (*big.Int, error) {
	if subnetEVMChainID != 0 {
		return new(big.Int).SetUint64(subnetEVMChainID), nil
	}
	ux.Logger.PrintToUser("Enter your subnet's ChainId. It can be any positive integer.")
	return app.Prompt.CapturePositiveBigInt("ChainId")
}

func getTokenName(app *application.Avalanche, subnetEVMTokenName string) (string, error) {
	if subnetEVMTokenName != "" {
		return subnetEVMTokenName, nil
	}
	ux.Logger.PrintToUser("Select a symbol for your subnet's native token")
	tokenName, err := app.Prompt.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}

	return tokenName, nil
}

func getDescriptors(
	app *application.Avalanche,
	subnetEVMChainID uint64,
	subnetEVMTokenName string,
) (
	*big.Int,
	string,
	statemachine.StateDirection,
	error,
) {
	chainID, err := getChainID(app, subnetEVMChainID)
	if err != nil {
		return nil, "", statemachine.Stop, err
	}

	tokenName, err := getTokenName(app, subnetEVMTokenName)
	if err != nil {
		return nil, "", statemachine.Stop, err
	}

	return chainID, tokenName, statemachine.Forward, nil
}
