// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/keychain"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
)

func promptOwners(
	kc *keychain.Keychain,
	controlKeys []string,
	sameControlKey bool,
	threshold uint32,
	subnetAuthKeys []string,
	creatingBlockchain bool,
) ([]string, uint32, error) {
	var err error
	// accept only one control keys specification
	if len(controlKeys) > 0 && sameControlKey {
		return nil, 0, errMutuallyExlusiveControlKeys
	}
	// use first fee-paying key as control key
	if sameControlKey {
		kcKeys, err := kc.PChainFormattedStrAddresses()
		if err != nil {
			return nil, 0, err
		}
		if len(kcKeys) == 0 {
			return nil, 0, fmt.Errorf("no keys found on keychain")
		}
		controlKeys = kcKeys[:1]
	}
	// prompt for control keys
	if controlKeys == nil {
		var cancelled bool
		controlKeys, cancelled, err = getControlKeys(kc, creatingBlockchain)
		if err != nil {
			return nil, 0, err
		}
		if cancelled {
			ux.Logger.PrintToUser("User cancelled. No operation was performed")
			return nil, 0, fmt.Errorf("user cancelled operation")
		}
	}
	ux.Logger.PrintToUser("Your Subnet's control keys: %s", controlKeys)
	// validate and prompt for threshold
	if threshold == 0 && subnetAuthKeys != nil {
		threshold = uint32(len(subnetAuthKeys))
	}
	if threshold > uint32(len(controlKeys)) {
		return nil, 0, fmt.Errorf("given threshold is greater than number of control keys")
	}
	if threshold == 0 {
		threshold, err = getThreshold(len(controlKeys))
		if err != nil {
			return nil, 0, err
		}
	}
	return controlKeys, threshold, nil
}

func getControlKeys(kc *keychain.Keychain, creatingBlockchain bool) ([]string, bool, error) {
	controlKeysInitialPrompt := "Configure which addresses may make changes to the subnet.\n" +
		"These addresses are known as your control keys. You will also\n" +
		"set how many control keys are required to make a subnet change (the threshold)."
	ux.Logger.PrintToUser(controlKeysInitialPrompt)

	if creatingBlockchain {
		return getControlKeysForDeploy(kc)
	} else {
		return getControlKeysForChangeOwner(kc.Network)
	}
}

func getControlKeysForDeploy(kc *keychain.Keychain) ([]string, bool, error) {
	moreKeysPrompt := "How would you like to set your control keys?"

	const (
		useAll = "Use all stored keys"
		custom = "Custom list"
	)

	var feePaying string
	var listOptions []string
	if kc.UsesLedger {
		feePaying = "Use ledger address"
	} else {
		feePaying = "Use fee-paying key"
	}
	if kc.Network.Kind == models.Mainnet {
		listOptions = []string{feePaying, custom}
	} else {
		listOptions = []string{feePaying, useAll, custom}
	}

	listDecision, err := app.Prompt.CaptureList(moreKeysPrompt, listOptions)
	if err != nil {
		return nil, false, err
	}

	var (
		keys      []string
		cancelled bool
	)

	switch listDecision {
	case feePaying:
		var kcKeys []string
		kcKeys, err = kc.PChainFormattedStrAddresses()
		if err != nil {
			return nil, false, err
		}
		if len(kcKeys) == 0 {
			return nil, false, fmt.Errorf("no keys found on keychain")
		}
		keys = kcKeys[:1]
	case useAll:
		keys, err = useAllKeys(kc.Network)
	case custom:
		keys, cancelled, err = enterCustomKeys(kc.Network)
	}
	if err != nil {
		return nil, false, err
	}
	if cancelled {
		return nil, true, nil
	}
	return keys, false, nil
}

func getControlKeysForChangeOwner(network models.Network) ([]string, bool, error) {
	moreKeysPrompt := "Which control keys would you like to set as the new subnet owners?"

	const (
		getFromStored = "Get address from an existing stored key (created from avalanche key create or avalanche key import)"
		custom        = "Custom"
	)

	listOptions := []string{getFromStored, custom}

	listDecision, err := app.Prompt.CaptureList(moreKeysPrompt, listOptions)
	if err != nil {
		return nil, false, err
	}

	var (
		keys      []string
		cancelled bool
	)

	switch listDecision {
	case getFromStored:
		key, err := prompts.CaptureKeyAddress(
			app.Prompt,
			"be set as a subnet control key",
			app.GetKeyDir(),
			app.GetKey,
			network,
			prompts.PChainFormat,
		)
		if err != nil {
			return nil, false, err
		}
		keys = []string{key}
	case custom:
		keys, cancelled, err = enterCustomKeys(network)
	}
	if err != nil {
		return nil, false, err
	}
	if cancelled {
		return nil, true, nil
	}
	return keys, false, nil
}

func useAllKeys(network models.Network) ([]string, error) {
	existing := []string{}

	files, err := os.ReadDir(app.GetKeyDir())
	if err != nil {
		return nil, err
	}

	keyPaths := make([]string, 0, len(files))

	for _, f := range files {
		if strings.HasSuffix(f.Name(), constants.KeySuffix) {
			keyPaths = append(keyPaths, filepath.Join(app.GetKeyDir(), f.Name()))
		}
	}

	for _, kp := range keyPaths {
		k, err := key.LoadSoft(network.ID, kp)
		if err != nil {
			return nil, err
		}

		existing = append(existing, k.P()...)
	}

	return existing, nil
}

func enterCustomKeys(network models.Network) ([]string, bool, error) {
	controlKeysPrompt := "Enter control keys"
	for {
		// ask in a loop so that if some condition is not met we can keep asking
		controlKeys, cancelled, err := getAddrLoop(controlKeysPrompt, constants.ControlKey, network)
		if err != nil {
			return nil, false, err
		}
		if cancelled {
			return nil, cancelled, nil
		}
		if len(controlKeys) != 0 {
			return controlKeys, false, nil
		}
		ux.Logger.PrintToUser("This tool does not allow to proceed without any control key set")
	}
}

// getAddrLoop asks as many addresses the user requires, until Done or Cancel is selected
// TODO: add info for TokenMinter and ValidatorManagerController
func getAddrLoop(prompt, label string, network models.Network) ([]string, bool, error) {
	info := ""
	goal := ""
	switch label {
	case constants.ControlKey:
		info = "Control keys are P-Chain addresses which have admin rights on the subnet.\n" +
			"Only private keys which control such addresses are allowed to make changes on the subnet"
		goal = "be set as a subnet control key"
	case constants.TokenMinter:
		goal = "enable as new native token minter"
	case constants.ValidatorManagerController:
		goal = "enable as controller of ValidatorManager contract"
	default:
	}
	customPrompt := "Enter P-Chain address (Example: P-...)"
	addressFormat := prompts.PChainFormat
	if label != constants.ControlKey {
		customPrompt = "Enter address"
		addressFormat = prompts.EVMFormat
	}
	return prompts.CaptureListDecision(
		// we need this to be able to mock test
		app.Prompt,
		// the main prompt for entering address keys
		prompt,
		// the Capture function to use
		func(_ string) (string, error) {
			return prompts.PromptAddress(
				app.Prompt,
				goal,
				app.GetKeyDir(),
				app.GetKey,
				"",
				network,
				addressFormat,
				customPrompt,
			)
		},
		// the prompt for each address
		"",
		// label describes the entity we are prompting for (e.g. address, control key, etc.)
		label,
		// optional parameter to allow the user to print the info string for more information
		info,
	)
}

// getThreshold prompts for the threshold of addresses as a number
func getThreshold(maxLen int) (uint32, error) {
	if maxLen == 1 {
		return uint32(1), nil
	}
	// create a list of indexes so the user only has the option to choose what is the threshold
	// instead of entering
	indexList := make([]string, maxLen)
	for i := 0; i < maxLen; i++ {
		indexList[i] = strconv.Itoa(i + 1)
	}
	threshold, err := app.Prompt.CaptureList("Select required number of control key signatures to make a subnet change", indexList)
	if err != nil {
		return 0, err
	}
	intTh, err := strconv.ParseUint(threshold, 0, 32)
	if err != nil {
		return 0, err
	}
	// this now should technically not happen anymore, but let's leave it as a double stitch
	if intTh > uint64(maxLen) {
		return 0, fmt.Errorf("the threshold can't be bigger than the number of control keys")
	}
	return uint32(intTh), err
}

func getKeyForChangeOwner(previouslyUsedAddr string, network models.Network) (string, error) {
	moreKeysPrompt := "Which key would you like to set as change owner for leftover AVAX if the node is removed from validator set?"

	const (
		getFromStored = "Get address from an existing stored key (created from avalanche key create or avalanche key import)"
		custom        = "Custom"
	)
	previousAddres := fmt.Sprintf("Previously used address %s", previouslyUsedAddr)

	listOptions := []string{getFromStored, custom}
	if previouslyUsedAddr != "" {
		listOptions = []string{previousAddres, getFromStored, custom}
	}
	listDecision, err := app.Prompt.CaptureList(moreKeysPrompt, listOptions)
	if err != nil {
		return "", err
	}

	var key string

	switch listDecision {
	case previousAddres:
		key = previouslyUsedAddr
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
