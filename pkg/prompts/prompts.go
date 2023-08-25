// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
	"github.com/manifoldco/promptui"
	"golang.org/x/mod/semver"
)

const (
	Yes = "Yes"
	No  = "No"

	Add        = "Add"
	Del        = "Delete"
	Preview    = "Preview"
	MoreInfo   = "More Info"
	Done       = "Done"
	Cancel     = "Cancel"
	LessThanEq = "Less Than Or Eq"
	MoreThanEq = "More Than Or Eq"
	MoreThan   = "More Than"
)

var errNoKeys = errors.New("no keys")

type Comparator struct {
	Label string // Label that identifies reference value
	Type  string // Less Than Eq or More than Eq
	Value uint64 // Value to Compare To
}

func (comparator *Comparator) Validate(val uint64) error {
	switch comparator.Type {
	case LessThanEq:
		if val > comparator.Value {
			return fmt.Errorf(fmt.Sprintf("the value must be smaller than or equal to %s (%d)", comparator.Label, comparator.Value))
		}
	case MoreThan:
		if val <= comparator.Value {
			return fmt.Errorf(fmt.Sprintf("the value must be bigger than %s (%d)", comparator.Label, comparator.Value))
		}
	case MoreThanEq:
		if val < comparator.Value {
			return fmt.Errorf(fmt.Sprintf("the value must be bigger than or equal to %s (%d)", comparator.Label, comparator.Value))
		}
	}
	return nil
}

type Prompter interface {
	CapturePositiveBigInt(promptStr string) (*big.Int, error)
	CaptureAddress(promptStr string) (common.Address, error)
	CaptureNewFilepath(promptStr string) (string, error)
	CaptureExistingFilepath(promptStr string) (string, error)
	CaptureYesNo(promptStr string) (bool, error)
	CaptureNoYes(promptStr string) (bool, error)
	CaptureList(promptStr string, options []string) (string, error)
	CaptureString(promptStr string) (string, error)
	CaptureGitURL(promptStr string) (*url.URL, error)
	CaptureStringAllowEmpty(promptStr string) (string, error)
	CaptureEmail(promptStr string) (string, error)
	CaptureIndex(promptStr string, options []any) (int, error)
	CaptureVersion(promptStr string) (string, error)
	CaptureFujiDuration(promptStr string) (time.Duration, error)
	CaptureMainnetDuration(promptStr string) (time.Duration, error)
	CaptureDate(promptStr string) (time.Time, error)
	CaptureNodeID(promptStr string) (ids.NodeID, error)
	CaptureID(promptStr string) (ids.ID, error)
	CaptureWeight(promptStr string) (uint64, error)
	CapturePositiveInt(promptStr string, comparators []Comparator) (int, error)
	CaptureUint32(promptStr string) (uint32, error)
	CaptureUint64(promptStr string) (uint64, error)
	CaptureFloat(promptStr string, validator func(float64) error) (float64, error)
	CaptureUint64Compare(promptStr string, comparators []Comparator) (uint64, error)
	CapturePChainAddress(promptStr string, network models.Network) (string, error)
	CaptureFutureDate(promptStr string, minDate time.Time) (time.Time, error)
	ChooseKeyOrLedger(goal string) (bool, error)
}

type realPrompter struct{}

// NewProcessChecker creates a new process checker which can respond if the server is running
func NewPrompter() Prompter {
	return &realPrompter{}
}

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
			if contains(finalList, elem) {
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

func (*realPrompter) CaptureFujiDuration(promptStr string) (time.Duration, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateFujiStakingDuration,
	}

	durationStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(durationStr)
}

func (*realPrompter) CaptureMainnetDuration(promptStr string) (time.Duration, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateMainnetStakingDuration,
	}

	durationStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(durationStr)
}

func (*realPrompter) CaptureDate(promptStr string) (time.Time, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateTime,
	}

	timeStr, err := prompt.Run()
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse(constants.TimeParseLayout, timeStr)
}

func (*realPrompter) CaptureID(promptStr string) (ids.ID, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateID,
	}

	idStr, err := prompt.Run()
	if err != nil {
		return ids.Empty, err
	}
	return ids.FromString(idStr)
}

func (*realPrompter) CaptureNodeID(promptStr string) (ids.NodeID, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateNodeID,
	}

	nodeIDStr, err := prompt.Run()
	if err != nil {
		return ids.EmptyNodeID, err
	}
	return ids.NodeIDFromString(nodeIDStr)
}

func (*realPrompter) CaptureWeight(promptStr string) (uint64, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateWeight,
	}

	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(amountStr, 10, 64)
}

func (*realPrompter) CaptureUint32(promptStr string) (uint32, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(input string) error {
			_, err := strconv.ParseUint(input, 0, 32)
			if err != nil {
				return err
			}
			return nil
		},
	}
	input, err := prompt.Run()
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseUint(input, 0, 32)
	if err != nil {
		return 0, err
	}
	return uint32(val), nil
}

func (*realPrompter) CaptureUint64(promptStr string) (uint64, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateBiggerThanZero,
	}

	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(amountStr, 0, 64)
}

func (*realPrompter) CaptureFloat(promptStr string, validator func(float64) error) (float64, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(input string) error {
			val, err := strconv.ParseFloat(input, 64)
			if err != nil {
				return err
			}
			return validator(val)
		},
	}

	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(amountStr, 64)
}

func (*realPrompter) CapturePositiveInt(promptStr string, comparators []Comparator) (int, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(input string) error {
			val, err := strconv.Atoi(input)
			if err != nil {
				return err
			}
			if val < 0 {
				return errors.New("input is less than 0")
			}
			for _, comparator := range comparators {
				if err := comparator.Validate(uint64(val)); err != nil {
					return err
				}
			}
			return nil
		},
	}

	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(amountStr)
}

func (*realPrompter) CaptureUint64Compare(promptStr string, comparators []Comparator) (uint64, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(input string) error {
			val, err := strconv.ParseUint(input, 0, 64)
			if err != nil {
				return err
			}
			for _, comparator := range comparators {
				if err := comparator.Validate(val); err != nil {
					return err
				}
			}
			return nil
		},
	}

	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(amountStr, 0, 64)
}

func (*realPrompter) CapturePositiveBigInt(promptStr string) (*big.Int, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validatePositiveBigInt,
	}

	amountStr, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	amountInt := new(big.Int)
	amountInt, ok := amountInt.SetString(amountStr, 10)
	if !ok {
		return nil, errors.New("SetString: error")
	}
	return amountInt, nil
}

func (*realPrompter) CapturePChainAddress(promptStr string, network models.Network) (string, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: getPChainValidationFunc(network),
	}

	return prompt.Run()
}

func (*realPrompter) CaptureAddress(promptStr string) (common.Address, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateAddress,
	}

	addressStr, err := prompt.Run()
	if err != nil {
		return common.Address{}, err
	}

	addressHex := common.HexToAddress(addressStr)
	return addressHex, nil
}

func (*realPrompter) CaptureExistingFilepath(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateExistingFilepath,
	}

	pathStr, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return pathStr, nil
}

func (*realPrompter) CaptureNewFilepath(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateNewFilepath,
	}

	pathStr, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return pathStr, nil
}

func yesNoBase(promptStr string, orderedOptions []string) (bool, error) {
	prompt := promptui.Select{
		Label: promptStr,
		Items: orderedOptions,
	}

	_, decision, err := prompt.Run()
	if err != nil {
		return false, err
	}
	return decision == Yes, nil
}

func (*realPrompter) CaptureYesNo(promptStr string) (bool, error) {
	return yesNoBase(promptStr, []string{Yes, No})
}

func (*realPrompter) CaptureNoYes(promptStr string) (bool, error) {
	return yesNoBase(promptStr, []string{No, Yes})
}

func (*realPrompter) CaptureList(promptStr string, options []string) (string, error) {
	prompt := promptui.Select{
		Label: promptStr,
		Items: options,
	}
	_, listDecision, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return listDecision, nil
}

func (*realPrompter) CaptureEmail(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateEmail,
	}

	str, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureStringAllowEmpty(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
	}

	str, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureString(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(input string) error {
			if input == "" {
				return errors.New("string cannot be empty")
			}
			return nil
		},
	}

	str, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureGitURL(promptStr string) (*url.URL, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateURL,
	}

	str, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	parsedURL, err := url.ParseRequestURI(str)
	if err != nil {
		return nil, err
	}

	return parsedURL, nil
}

func (*realPrompter) CaptureVersion(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(input string) error {
			if !semver.IsValid(input) {
				return errors.New("version must be a legal semantic version (ex: v1.1.1)")
			}
			return nil
		},
	}

	str, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureIndex(promptStr string, options []any) (int, error) {
	prompt := promptui.Select{
		Label: promptStr,
		Items: options,
	}

	listIndex, _, err := prompt.Run()
	if err != nil {
		return 0, err
	}
	return listIndex, nil
}

// CaptureFutureDate requires from the user a date input which is in the future.
// If `minDate` is not empty, the minimum time in the future from the provided date is required
// Otherwise, time from time.Now() is chosen.
func (*realPrompter) CaptureFutureDate(promptStr string, minDate time.Time) (time.Time, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(s string) error {
			t, err := time.Parse(constants.TimeParseLayout, s)
			if err != nil {
				return err
			}
			if minDate == (time.Time{}) {
				minDate = time.Now()
			}
			if t.Before(minDate.UTC()) {
				return fmt.Errorf("the provided date is before %s UTC", minDate.Format(constants.TimeParseLayout))
			}
			return nil
		},
	}

	timestampStr, err := prompt.Run()
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse(constants.TimeParseLayout, timestampStr)
}

// returns true [resp. false] if user chooses stored key [resp. ledger] option
func (prompter *realPrompter) ChooseKeyOrLedger(goal string) (bool, error) {
	const (
		keyOption    = "Use stored key"
		ledgerOption = "Use ledger"
	)
	option, err := prompter.CaptureList(
		fmt.Sprintf("Which key source should be used to %s?", goal),
		[]string{keyOption, ledgerOption},
	)
	if err != nil {
		return false, err
	}
	return option == keyOption, nil
}

func contains[T comparable](list []T, element T) bool {
	for _, val := range list {
		if val == element {
			return true
		}
	}
	return false
}

func getIndexInSlice[T comparable](list []T, element T) (int, error) {
	for i, val := range list {
		if val == element {
			return i, nil
		}
	}
	return 0, fmt.Errorf("element not found")
}

// check subnet authorization criteria:
// - [subnetAuthKeys] satisfy subnet's [threshold]
// - [subnetAuthKeys] is a subset of subnet's [controlKeys]
func CheckSubnetAuthKeys(subnetAuthKeys []string, controlKeys []string, threshold uint32) error {
	if len(subnetAuthKeys) != int(threshold) {
		return fmt.Errorf("number of given subnet auth differs from the threshold")
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
			return fmt.Errorf("subnet auth key %s does not belong to control keys", subnetAuthKey)
		}
	}
	return nil
}

// get subnet authorization keys from the user, as a subset of the subnet's [controlKeys]
// with a len equal to the subnet's [threshold]
func GetSubnetAuthKeys(prompt Prompter, controlKeys []string, threshold uint32) ([]string, error) {
	if len(controlKeys) == int(threshold) {
		return controlKeys, nil
	}
	subnetAuthKeys := []string{}
	filteredControlKeys := []string{}
	filteredControlKeys = append(filteredControlKeys, controlKeys...)
	for len(subnetAuthKeys) != int(threshold) {
		subnetAuthKey, err := prompt.CaptureList(
			"Choose a subnet auth key",
			filteredControlKeys,
		)
		if err != nil {
			return nil, err
		}
		index, err := getIndexInSlice(filteredControlKeys, subnetAuthKey)
		if err != nil {
			return nil, err
		}
		subnetAuthKeys = append(subnetAuthKeys, subnetAuthKey)
		filteredControlKeys = append(filteredControlKeys[:index], filteredControlKeys[index+1:]...)
	}
	return subnetAuthKeys, nil
}

func GetFujiKeyOrLedger(prompt Prompter, goal string, keyDir string) (bool, string, error) {
	useStoredKey, err := prompt.ChooseKeyOrLedger(goal)
	if err != nil {
		return false, "", err
	}
	if !useStoredKey {
		return true, "", nil
	}
	keyName, err := captureKeyName(prompt, goal, keyDir)
	if err != nil {
		if errors.Is(err, errNoKeys) {
			ux.Logger.PrintToUser("No private keys have been found. Create a new one with `avalanche key create`")
		}
		return false, "", err
	}
	return false, keyName, nil
}

func captureKeyName(prompt Prompter, goal string, keyDir string) (string, error) {
	files, err := os.ReadDir(keyDir)
	if err != nil {
		return "", err
	}

	if len(files) < 1 {
		return "", errNoKeys
	}

	keys := []string{}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), constants.KeySuffix) {
			keys = append(keys, strings.TrimSuffix(f.Name(), constants.KeySuffix))
		}
	}

	keyName, err := prompt.CaptureList(fmt.Sprintf("Which stored key should be used to %s?", goal), keys)
	if err != nil {
		return "", err
	}

	return keyName, nil
}
