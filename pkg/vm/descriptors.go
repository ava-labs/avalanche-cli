// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func getChainID(app *application.Avalanche) (*big.Int, error) {
	// TODO check against known chain ids and provide warning
	ux.Logger.PrintToUser("Enter your subnet's ChainId. It can be any positive integer.")

	chainID, err := app.Prompt.CapturePositiveBigInt("ChainId")
	if err != nil {
		return nil, err
	}

	exists, err := app.ChainIDExists(chainID.String())
	if err != nil {
		return nil, err
	}
	if exists {
		ux.Logger.PrintToUser("The provided chain ID %q already exists! Try a different one:", chainID.String())
		return getChainID(app)
	}

	return chainID, nil
}

func getTokenName(app *application.Avalanche) (string, error) {
	ux.Logger.PrintToUser("Select a symbol for your subnet's native token")
	tokenName, err := app.Prompt.CaptureString("Token symbol")
	if err != nil {
		return "", err
	}

	return tokenName, nil
}

func getVMVersion(
	app *application.Avalanche,
	vmName string,
	repoName string,
	vmVersion string,
	addGoBackOption bool,
) (string, stateDirection, error) {
	var err error
	direction := forward
	if vmVersion == "latest" {
		vmVersion, err = binutils.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
			constants.AvaLabsOrg,
			repoName,
		))
		if err != nil {
			return "", stop, err
		}
	} else if vmVersion == "" {
		vmVersion, direction, err = getManualVMVersion(app, vmName, repoName, addGoBackOption)
		if err != nil {
			return "", stop, err
		}
	}
	return vmVersion, direction, nil
}

func getManualVMVersion(
	app *application.Avalanche,
	vmName string,
	repoName string,
	addGoBackOption bool,
) (string, stateDirection, error) {
	const (
		useLatest = "Use latest version"
		useCustom = "Specify custom version"
	)
	defaultPrompt := fmt.Sprintf("What version of %s would you like?", vmName)

	versionOptions := []string{useLatest, useCustom}
	if addGoBackOption {
		versionOptions = append(versionOptions, goBackMsg)
	}

	versionOption, err := app.Prompt.CaptureList(
		defaultPrompt,
		versionOptions,
	)
	if err != nil {
		return "", stop, err
	}

	if versionOption == goBackMsg {
		return "", backward, err
	}

	if versionOption == useLatest {
		// Get and return latest version
		version, err := binutils.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
			constants.AvaLabsOrg,
			repoName,
		))
		return version, forward, err
	}

	// prompt for version
	version, err := app.Prompt.CaptureVersion(fmt.Sprintf("%s version", vmName))
	if err != nil {
		return "", stop, err
	}

	return version, forward, nil
}

func getDescriptors(app *application.Avalanche, subnetEVMVersion string) (*big.Int, string, string, stateDirection, error) {
	chainID, err := getChainID(app)
	if err != nil {
		return nil, "", "", stop, err
	}

	tokenName, err := getTokenName(app)
	if err != nil {
		return nil, "", "", stop, err
	}

	subnetEVMVersion, _, err = getVMVersion(app, "Subnet-EVM", constants.SubnetEVMRepoName, subnetEVMVersion, false)
	if err != nil {
		return nil, "", "", stop, err
	}

	return chainID, tokenName, subnetEVMVersion, forward, nil
}
