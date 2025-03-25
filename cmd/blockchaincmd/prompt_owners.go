// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"fmt"
	"github.com/ava-labs/avalanche-cli/cmd/flags"
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
	subnetFlags *flags.SubnetFlags,
	creatingBlockchain bool,
) error {
	var err error
	// accept only one control keys specification
	if len(subnetFlags.ControlKeys) > 0 && subnetFlags.SameControlKey {
		return errMutuallyExlusiveControlKeys
	}
	// use first fee-paying key as control key
	if subnetFlags.SameControlKey {
		kcKeys, err := kc.PChainFormattedStrAddresses()
		if err != nil {
			return err
		}
		if len(kcKeys) == 0 {
			return fmt.Errorf("no keys found on keychain")
		}
		subnetFlags.ControlKeys = kcKeys[:1]
	}
	// prompt for control keys
	if subnetFlags.ControlKeys == nil {
		var cancelled bool
		subnetFlags.ControlKeys, cancelled, err = getControlKeys(kc, creatingBlockchain)
		if err != nil {
			return err
		}
		if cancelled {
			ux.Logger.PrintToUser("User cancelled. No operation was performed")
			return fmt.Errorf("user cancelled operation")
		}
	}
	ux.Logger.PrintToUser("Your blockchain control keys: %s", subnetFlags.ControlKeys)
	// validate and prompt for threshold
	if subnetFlags.Threshold == 0 && subnetFlags.SubnetAuthKeys != nil {
		subnetFlags.Threshold = uint32(len(subnetFlags.SubnetAuthKeys))
	}
	if subnetFlags.Threshold > uint32(len(subnetFlags.ControlKeys)) {
		return fmt.Errorf("given threshold is greater than number of control keys")
	}
	if subnetFlags.Threshold == 0 {
		subnetFlags.Threshold, err = getThreshold(len(subnetFlags.ControlKeys))
		if err != nil {
			return err
		}
	}
	return nil
}

func getControlKeys(kc *keychain.Keychain, creatingBlockchain bool) ([]string, bool, error) {
	controlKeysInitialPrompt := "Configure which addresses may make changes to the blockchain.\n" +
		"These addresses are known as your control keys. You will also\n" +
		"set how many control keys are required to make a blockchain change (the threshold)."
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
	moreKeysPrompt := "Which control keys would you like to set as the new blockchain owners?"

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
			"be set as a control key",
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
		controlKeys, cancelled, err := controlKeysLoop(controlKeysPrompt, network)
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

// controlKeysLoop asks as many controlkeys the user requires, until Done or Cancel is selected
func controlKeysLoop(controlKeysPrompt string, network models.Network) ([]string, bool, error) {
	label := "Control key"
	info := "Control keys are P-Chain addresses which have admin rights on the subnet.\n" +
		"Only private keys which control such addresses are allowed to make changes on the subnet"
	customPrompt := "Enter P-Chain address (Example: P-...)"
	return prompts.CaptureListDecision(
		// we need this to be able to mock test
		app.Prompt,
		// the main prompt for entering address keys
		controlKeysPrompt,
		// the Capture function to use
		func(_ string) (string, error) {
			return prompts.PromptAddress(
				app.Prompt,
				"be set as a control key",
				app.GetKeyDir(),
				app.GetKey,
				"",
				network,
				prompts.PChainFormat,
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
	threshold, err := app.Prompt.CaptureList("Select required number of control key signatures to make a blockchain change", indexList)
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
