// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"

	"github.com/spf13/cobra"
)

type AddressFormat int64

const (
	Undefined AddressFormat = iota
	PChainFormat
	EVMFormat
	XChainFormat
)

const (
	Add          = "Add"
	Del          = "Delete"
	Preview      = "Preview"
	MoreInfo     = "More Info"
	Done         = "Done"
	Cancel       = "Cancel"
	customOption = "Custom"
)

var errNoKeys = errors.New("no keys")

// CaptureListDecision runs a for loop and continuously asks the
// user for a specific input (currently only `CapturePChainAddress`
// and `CaptureAddress` is supported) until the user cancels or
// chooses `Done`. It does also offer an optional `info` to print
// (if provided) and a preview. Items can also be removed.
func CaptureListDecision[T comparable](
	// we need this in order to be able to run mock tests
	prompter Prompter,
	// the main prompt for entering address keys
	prompt string,
	// the Capture function to use
	capture func(prompt string) (T, error),
	// the prompt for each address
	capturePrompt string,
	// label describes the entity we are prompting for (e.g. address, control key, etc.)
	label string,
	// optional parameter to allow the user to print the info string for more information
	info string,
) ([]T, bool, error) {
	finalList := []T{}
	for {
		listDecision, err := prompter.CaptureList(
			prompt, []string{Add, Del, Preview, MoreInfo, Done, Cancel},
		)
		if err != nil {
			return nil, false, err
		}
		switch listDecision {
		case Add:
			elem, err := capture(capturePrompt)
			if err != nil {
				return nil, false, err
			}
			if slices.Contains(finalList, elem) {
				fmt.Println(label + " already in list")
				continue
			}
			finalList = append(finalList, elem)
		case Del:
			if len(finalList) == 0 {
				fmt.Println("No " + label + " added yet")
				continue
			}
			finalListAnyT := []any{}
			for _, v := range finalList {
				finalListAnyT = append(finalListAnyT, v)
			}
			index, err := prompter.CaptureIndex("Choose element to remove:", finalListAnyT)
			if err != nil {
				return nil, false, err
			}
			finalList = append(finalList[:index], finalList[index+1:]...)
		case Preview:
			if len(finalList) == 0 {
				fmt.Println("The list is empty")
				break
			}
			for i, k := range finalList {
				fmt.Printf("%d. %v\n", i, k)
			}
		case MoreInfo:
			if info != "" {
				fmt.Println(info)
			}
		case Done:
			return finalList, false, nil
		case Cancel:
			return nil, true, nil
		default:
			return nil, false, errors.New("unexpected option")
		}
	}
}

// check subnet authorization criteria:
// - [subnetAuthKeys] satisfy subnet's [threshold]
// - [subnetAuthKeys] is a subset of subnet's [controlKeys]
func CheckSubnetAuthKeys(walletKeys []string, subnetAuthKeys []string, controlKeys []string, threshold uint32) error {
	for _, walletKey := range walletKeys {
		if slices.Contains(controlKeys, walletKey) && !slices.Contains(subnetAuthKeys, walletKey) {
			return fmt.Errorf("wallet key %s is a control key so it must be included in auth keys", walletKey)
		}
	}
	if len(subnetAuthKeys) != int(threshold) {
		return fmt.Errorf("number of given auth keys differs from the threshold")
	}
	for _, subnetAuthKey := range subnetAuthKeys {
		found := false
		for _, controlKey := range controlKeys {
			if subnetAuthKey == controlKey {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("auth key %s does not belong to control keys", subnetAuthKey)
		}
	}
	return nil
}

// get subnet authorization keys from the user, as a subset of the subnet's [controlKeys]
// with a len equal to the subnet's [threshold]
func GetSubnetAuthKeys(prompter Prompter, walletKeys []string, controlKeys []string, threshold uint32) ([]string, error) {
	if len(controlKeys) == int(threshold) {
		return controlKeys, nil
	}
	subnetAuthKeys := []string{}
	filteredControlKeys := []string{}
	filteredControlKeys = append(filteredControlKeys, controlKeys...)
	for _, walletKey := range walletKeys {
		if slices.Contains(controlKeys, walletKey) {
			ux.Logger.PrintToUser("Adding wallet key %s to the tx auth keys as it is a control key", walletKey)
			subnetAuthKeys = append(subnetAuthKeys, walletKey)
			index, err := utils.GetIndexInSlice(filteredControlKeys, walletKey)
			if err != nil {
				return nil, err
			}
			filteredControlKeys = append(filteredControlKeys[:index], filteredControlKeys[index+1:]...)
		}
	}
	for len(subnetAuthKeys) != int(threshold) {
		subnetAuthKey, err := prompter.CaptureList(
			"Choose an auth key",
			filteredControlKeys,
		)
		if err != nil {
			return nil, err
		}
		index, err := utils.GetIndexInSlice(filteredControlKeys, subnetAuthKey)
		if err != nil {
			return nil, err
		}
		subnetAuthKeys = append(subnetAuthKeys, subnetAuthKey)
		filteredControlKeys = append(filteredControlKeys[:index], filteredControlKeys[index+1:]...)
	}
	return subnetAuthKeys, nil
}

func GetKeyOrLedger(prompter Prompter, goal string, keyDir string, includeEwoq bool) (bool, string, error) {
	useStoredKey, err := prompter.ChooseKeyOrLedger(goal)
	if err != nil {
		return false, "", err
	}
	if !useStoredKey {
		return true, "", nil
	}
	keyName, err := CaptureKeyName(prompter, goal, keyDir, includeEwoq)
	if err != nil {
		if errors.Is(err, errNoKeys) {
			ux.Logger.PrintToUser("No private keys have been found. Create a new one with `avalanche key create`")
		}
		return false, "", err
	}
	return false, keyName, nil
}

func CaptureKeyName(prompter Prompter, goal string, keyDir string, includeEwoq bool) (string, error) {
	keyNames, err := utils.GetKeyNames(keyDir, includeEwoq)
	if err != nil {
		return "", err
	}
	if len(keyNames) == 0 {
		return "", errNoKeys
	}
	size := len(keyNames)
	if size > 10 {
		size = 10
	}
	keyName, err := prompter.CaptureListWithSize(fmt.Sprintf("Which stored key should be used %s?", goal), keyNames, size)
	if err != nil {
		return "", err
	}
	return keyName, nil
}

func CaptureBoolFlag(
	prompter Prompter,
	cmd *cobra.Command,
	flagName string,
	flagValue bool,
	promptMsg string,
) (bool, error) {
	if flagValue {
		return true, nil
	}
	if flag := cmd.Flags().Lookup(flagName); flag == nil || !flag.Changed {
		return prompter.CaptureYesNo(promptMsg)
	} else {
		return cmd.Flags().GetBool(flagName)
	}
}

func PromptChain(
	prompter Prompter,
	prompt string,
	subnetNames []string,
	includePChain bool,
	includeXChain bool,
	includeCChain bool,
	avoidBlockchainName string,
	includeCustom bool,
) (bool, bool, bool, bool, string, string, error) {
	pChainOption := "P-Chain"
	xChainOption := "X-Chain"
	cChainOption := "C-Chain"
	notListedOption := "My blockchain isn't listed"
	subnetOptions := []string{}
	if includePChain {
		subnetOptions = append(subnetOptions, pChainOption)
	}
	if includeXChain {
		subnetOptions = append(subnetOptions, xChainOption)
	}
	if includeCChain {
		subnetOptions = append(subnetOptions, cChainOption)
	}
	subnetNames = utils.RemoveFromSlice(subnetNames, avoidBlockchainName)
	subnetOptions = append(subnetOptions, sdkutils.Map(subnetNames, func(s string) string { return "Blockchain " + s })...)
	if includeCustom {
		subnetOptions = append(subnetOptions, customOption)
	} else {
		subnetOptions = append(subnetOptions, notListedOption)
	}
	subnetOption, err := prompter.CaptureListWithSize(
		prompt,
		subnetOptions,
		11,
	)
	if err != nil {
		return false, false, false, false, "", "", err
	}
	if subnetOption == customOption {
		blockchainID, err := prompter.CaptureString("Blockchain ID/Alias")
		if err != nil {
			return false, false, false, false, "", "", err
		}
		return false, false, false, false, "", blockchainID, nil
	}
	if subnetOption == notListedOption {
		ux.Logger.PrintToUser("Please import the blockchain first, using the `avalanche blockchain import` command suite")
		return true, false, false, false, "", "", nil
	}
	switch subnetOption {
	case pChainOption:
		return false, true, false, false, "", "", nil
	case xChainOption:
		return false, false, true, false, "", "", nil
	case cChainOption:
		return false, false, false, true, "", "", nil
	default:
		return false, false, false, false, strings.TrimPrefix(subnetOption, "Blockchain "), "", nil
	}
}

func PromptPrivateKey(
	prompter Prompter,
	goal string,
	keyDir string,
	getKey func(string, models.Network, bool) (*key.SoftKey, error),
	genesisAddress string,
	genesisPrivateKey string,
) (string, error) {
	privateKey := ""
	cliKeyOpt := "Get private key from an existing stored key (created from avalanche key create or avalanche key import)"
	genesisKeyOpt := fmt.Sprintf("Use the private key of the Genesis Allocated address %s", genesisAddress)
	keyOptions := []string{cliKeyOpt, customOption}
	if genesisPrivateKey != "" {
		keyOptions = []string{genesisKeyOpt, cliKeyOpt, customOption}
	}
	keyOption, err := prompter.CaptureList(
		fmt.Sprintf("Which private key do you want to use to %s?", goal),
		keyOptions,
	)
	if err != nil {
		return "", err
	}
	switch keyOption {
	case cliKeyOpt:
		keyName, err := CaptureKeyName(prompter, goal, keyDir, true)
		if err != nil {
			return "", err
		}
		k, err := getKey(keyName, models.NewLocalNetwork(), false)
		if err != nil {
			return "", err
		}
		privateKey = k.PrivKeyHex()
	case customOption:
		privateKey, err = prompter.CaptureString("Private Key")
		if err != nil {
			return "", err
		}
	case genesisKeyOpt:
		privateKey = genesisPrivateKey
	}
	return privateKey, nil
}

func PromptAddress(
	prompter Prompter,
	goal string,
	keyDir string,
	getKey func(string, models.Network, bool) (*key.SoftKey, error),
	genesisAddress string,
	network models.Network,
	format AddressFormat,
	customPrompt string,
) (string, error) {
	address := ""
	cliKeyOpt := "Get address from an existing stored key (created from avalanche key create or avalanche key import)"
	genesisKeyOpt := fmt.Sprintf("Use the Genesis Allocated address %s", genesisAddress)
	keyOptions := []string{cliKeyOpt, customOption}
	if genesisAddress != "" {
		keyOptions = []string{genesisKeyOpt, cliKeyOpt, customOption}
	}
	keyOption, err := prompter.CaptureList(
		fmt.Sprintf("Which address do you want to %s?", goal),
		keyOptions,
	)
	if err != nil {
		return "", err
	}
	switch keyOption {
	case cliKeyOpt:
		address, err = CaptureKeyAddress(
			prompter,
			goal,
			keyDir,
			getKey,
			network,
			format,
		)
		if err != nil {
			return "", err
		}
	case customOption:
		switch format {
		case PChainFormat:
			address, err = prompter.CapturePChainAddress(customPrompt, network)
			if err != nil {
				return "", err
			}
		case XChainFormat:
			address, err = prompter.CaptureXChainAddress(customPrompt, network)
			if err != nil {
				return "", err
			}
		case EVMFormat:
			addr, err := prompter.CaptureAddress(customPrompt)
			if err != nil {
				return "", err
			}
			address = addr.Hex()
		}
	case genesisKeyOpt:
		address = genesisAddress
	}
	return address, nil
}

func CaptureKeyAddress(
	prompter Prompter,
	goal string,
	keyDir string,
	getKey func(string, models.Network, bool) (*key.SoftKey, error),
	network models.Network,
	format AddressFormat,
) (string, error) {
	includeEwoq := true
	if network.Kind == models.Fuji {
		includeEwoq = false
	}
	keyName, err := CaptureKeyName(prompter, goal, keyDir, includeEwoq)
	if err != nil {
		return "", err
	}
	k, err := getKey(keyName, network, false)
	if err != nil {
		return "", err
	}
	switch format {
	case PChainFormat:
		return k.P()[0], nil
	case XChainFormat:
		return k.X()[0], nil
	case EVMFormat:
		return k.C(), nil
	}
	return "", nil
}
