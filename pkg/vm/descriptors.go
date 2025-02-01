// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func getTokenSymbol(app *application.Avalanche, subnetEVMTokenSymbol string) (string, error) {
	if subnetEVMTokenSymbol != "" {
		return subnetEVMTokenSymbol, nil
	}
	ux.Logger.PrintToUser("Select a symbol for your blockchain native token")
	tokenSymbol, err := app.Prompt.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}

	return tokenSymbol, nil
}
