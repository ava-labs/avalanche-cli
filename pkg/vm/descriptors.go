// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
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

func getVMVersion(
	app *application.Avalanche,
	vmName string,
	repoName string,
	vmVersion string,
	addGoBackOption bool,
) (string, error) {
	var err error
	if vmVersion == "latest" {
		vmVersion, err = app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
			constants.AvaLabsOrg,
			repoName,
		))
		if err != nil {
			return "", err
		}
	} else if vmVersion == "" {
		vmVersion, _, err = askForVMVersion(app, vmName, repoName, addGoBackOption)
		if err != nil {
			return "", err
		}
	}
	return vmVersion, nil
}

func askForVMVersion(
	app *application.Avalanche,
	vmName string,
	repoName string,
	addGoBackOption bool,
) (string, statemachine.StateDirection, error) {
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
		return "", statemachine.Stop, err
	}

	if versionOption == goBackMsg {
		return "", statemachine.Backward, err
	}

	if versionOption == useLatest {
		// Get and return latest version
		version, err := app.Downloader.GetLatestReleaseVersion(binutils.GetGithubLatestReleaseURL(
			constants.AvaLabsOrg,
			repoName,
		))
		return version, statemachine.Forward, err
	}

	// prompt for version
	versions, err := app.Downloader.GetAllReleasesForRepo(constants.AvaLabsOrg, constants.SubnetEVMRepoName)
	if err != nil {
		return "", statemachine.Stop, err
	}
	version, err := app.Prompt.CaptureList("Pick the version for this VM", versions)
	if err != nil {
		return "", statemachine.Stop, err
	}

	return version, statemachine.Forward, nil
}

func getDescriptors(
	app *application.Avalanche,
	subnetEVMVersion string,
	subnetEVMChainID uint64,
	subnetEVMTokenName string,
) (
	*big.Int,
	string,
	string,
	statemachine.StateDirection,
	error,
) {
	chainID, err := getChainID(app, subnetEVMChainID)
	if err != nil {
		return nil, "", "", statemachine.Stop, err
	}

	tokenName, err := getTokenName(app, subnetEVMTokenName)
	if err != nil {
		return nil, "", "", statemachine.Stop, err
	}

	subnetEVMVersion, err = getVMVersion(app, "Subnet-EVM", constants.SubnetEVMRepoName, subnetEVMVersion, false)
	if err != nil {
		return nil, "", "", statemachine.Stop, err
	}

	return chainID, tokenName, subnetEVMVersion, statemachine.Forward, nil
}
