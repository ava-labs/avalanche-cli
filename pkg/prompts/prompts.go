// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"bufio"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/key"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	sdkutils "github.com/ava-labs/avalanche-cli/sdk/utils"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ethereum/go-ethereum/common"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"golang.org/x/mod/semver"
)

type AddressFormat int64

const (
	Undefined AddressFormat = iota
	PChainFormat
	EVMFormat
	XChainFormat
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
	NotEq      = "Not Eq"

	customOption = "Custom"
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
			return fmt.Errorf("the value must be smaller than or equal to %s (%d)", comparator.Label, comparator.Value)
		}
	case MoreThan:
		if val <= comparator.Value {
			return fmt.Errorf("the value must be bigger than %s (%d)", comparator.Label, comparator.Value)
		}
	case MoreThanEq:
		if val < comparator.Value {
			return fmt.Errorf("the value must be bigger than or equal to %s (%d)", comparator.Label, comparator.Value)
		}
	case NotEq:
		if val == comparator.Value {
			return fmt.Errorf("the value must be different than %s (%d)", comparator.Label, comparator.Value)
		}
	}
	return nil
}

type Prompter interface {
	CapturePositiveBigInt(promptStr string) (*big.Int, error)
	CaptureAddress(promptStr string) (common.Address, error)
	CaptureAddresses(promptStr string) ([]common.Address, error)
	CaptureNewFilepath(promptStr string) (string, error)
	CaptureExistingFilepath(promptStr string) (string, error)
	CaptureYesNo(promptStr string) (bool, error)
	CaptureNoYes(promptStr string) (bool, error)
	CaptureList(promptStr string, options []string) (string, error)
	CaptureListWithSize(promptStr string, options []string, size int) (string, error)
	CaptureString(promptStr string) (string, error)
	CaptureValidatedString(promptStr string, validator func(string) error) (string, error)
	CaptureURL(promptStr string, validateConnection bool) (string, error)
	CaptureRepoBranch(promptStr string, repo string) (string, error)
	CaptureRepoFile(promptStr string, repo string, branch string) (string, error)
	CaptureGitURL(promptStr string) (*url.URL, error)
	CaptureStringAllowEmpty(promptStr string) (string, error)
	CaptureEmail(promptStr string) (string, error)
	CaptureIndex(promptStr string, options []any) (int, error)
	CaptureVersion(promptStr string) (string, error)
	CaptureDuration(promptStr string) (time.Duration, error)
	CaptureFujiDuration(promptStr string) (time.Duration, error)
	CaptureMainnetDuration(promptStr string) (time.Duration, error)
	CaptureMainnetL1StakingDuration(promptStr string) (time.Duration, error)
	CaptureDate(promptStr string) (time.Time, error)
	CaptureNodeID(promptStr string) (ids.NodeID, error)
	CaptureID(promptStr string) (ids.ID, error)
	CaptureWeight(promptStr string, validator func(uint64) error) (uint64, error)
	CaptureValidatorBalance(promptStr string, availableBalance float64, minBalance float64) (float64, error)
	CapturePositiveInt(promptStr string, comparators []Comparator) (int, error)
	CaptureInt(promptStr string, validator func(int) error) (int, error)
	CaptureUint8(promptStr string) (uint8, error)
	CaptureUint16(promptStr string) (uint16, error)
	CaptureUint32(promptStr string) (uint32, error)
	CaptureUint64(promptStr string) (uint64, error)
	CaptureFloat(promptStr string, validator func(float64) error) (float64, error)
	CaptureUint64Compare(promptStr string, comparators []Comparator) (uint64, error)
	CapturePChainAddress(promptStr string, network models.Network) (string, error)
	CaptureXChainAddress(promptStr string, network models.Network) (string, error)
	CaptureFutureDate(promptStr string, minDate time.Time) (time.Time, error)
	ChooseKeyOrLedger(goal string) (bool, error)
}

type realPrompter struct{}

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

func (*realPrompter) CaptureDuration(promptStr string) (time.Duration, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: validateDuration,
	}

	durationStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(durationStr)
}

func (*realPrompter) CaptureFujiDuration(promptStr string) (time.Duration, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
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
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: validateMainnetStakingDuration,
	}

	durationStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(durationStr)
}

func (*realPrompter) CaptureMainnetL1StakingDuration(promptStr string) (time.Duration, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: validateMainnetL1StakingDuration,
	}

	durationStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(durationStr)
}

func (*realPrompter) CaptureDate(promptStr string) (time.Time, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
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
		Stdout:   ux.Logger,
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
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: ValidateNodeID,
	}

	nodeIDStr, err := prompt.Run()
	if err != nil {
		return ids.EmptyNodeID, err
	}
	return ids.NodeIDFromString(nodeIDStr)
}

// CaptureValidatorBalance captures balance in AVAX
func (*realPrompter) CaptureValidatorBalance(
	promptStr string,
	availableBalance float64,
	minBalance float64,
) (float64, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: validateValidatorBalanceFunc(availableBalance, minBalance),
	}
	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	amountFloat, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0, err
	}

	return amountFloat, nil
}

func (*realPrompter) CaptureWeight(promptStr string, validator func(uint64) error) (uint64, error) {
	prompt := promptui.Prompt{
		Stdout: ux.Logger,
		Label:  promptStr,
		Validate: func(input string) error {
			if err := validateWeight(input); err != nil {
				return err
			}
			val, err := strconv.ParseUint(input, 10, 64)
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

	return strconv.ParseUint(amountStr, 10, 64)
}

func (*realPrompter) CaptureInt(promptStr string, validator func(int) error) (int, error) {
	prompt := promptui.Prompt{
		Stdout: ux.Logger,
		Label:  promptStr,
		Validate: func(input string) error {
			val, err := strconv.Atoi(input)
			if err != nil {
				return err
			}
			return validator(val)
		},
	}
	input, err := prompt.Run()
	if err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(input)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func (*realPrompter) CaptureUint8(promptStr string) (uint8, error) {
	prompt := promptui.Prompt{
		Stdout: ux.Logger,
		Label:  promptStr,
		Validate: func(input string) error {
			_, err := strconv.ParseUint(input, 0, 8)
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
	val, err := strconv.ParseUint(input, 0, 8)
	if err != nil {
		return 0, err
	}
	return uint8(val), nil
}

func (*realPrompter) CaptureUint16(promptStr string) (uint16, error) {
	prompt := promptui.Prompt{
		Stdout: ux.Logger,
		Label:  promptStr,
		Validate: func(input string) error {
			_, err := strconv.ParseUint(input, 0, 16)
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
	val, err := strconv.ParseUint(input, 0, 16)
	if err != nil {
		return 0, err
	}
	return uint16(val), nil
}

func (*realPrompter) CaptureUint32(promptStr string) (uint32, error) {
	prompt := promptui.Prompt{
		Stdout: ux.Logger,
		Label:  promptStr,
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
		Stdout:   ux.Logger,
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
		Stdout: ux.Logger,
		Label:  promptStr,
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
		Stdout: ux.Logger,
		Label:  promptStr,
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
		Stdout: ux.Logger,
		Label:  promptStr,
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
		Stdout:   ux.Logger,
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
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: getPChainValidationFunc(network),
	}

	return prompt.Run()
}

func (*realPrompter) CaptureXChainAddress(promptStr string, network models.Network) (string, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: getXChainValidationFunc(network),
	}

	return prompt.Run()
}

func (*realPrompter) CaptureAddress(promptStr string) (common.Address, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: ValidateAddress,
	}

	addressStr, err := prompt.Run()
	if err != nil {
		return common.Address{}, err
	}

	addressHex := common.HexToAddress(addressStr)
	return addressHex, nil
}

func (*realPrompter) CaptureAddresses(promptStr string) ([]common.Address, error) {
	addressesStr := ""
	validated := false
	for !validated {
		var err error
		addressesStr, err = PromptLongString(promptui.IconGood + " " + promptStr + " ")
		if err != nil {
			return nil, err
		}
		if err := validateAddresses(addressesStr); err != nil {
			fmt.Println(err)
		} else {
			validated = true
		}
	}
	addresses := sdkutils.Map(
		strings.Split(addressesStr, ","),
		func(s string) common.Address {
			return common.HexToAddress(strings.TrimSpace(s))
		},
	)
	return addresses, nil
}

func (*realPrompter) CaptureExistingFilepath(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: validateExistingFilepath,
	}

	pathStr, err := prompt.Run()
	if err != nil {
		return "", err
	}
	pathStr = utils.ExpandHome(pathStr)

	return pathStr, nil
}

func (*realPrompter) CaptureNewFilepath(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
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
		Stdout: ux.Logger,
		Label:  promptStr,
		Items:  orderedOptions,
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
		Stdout: ux.Logger,
		Label:  promptStr,
		Items:  options,
	}
	_, listDecision, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return listDecision, nil
}

func (*realPrompter) CaptureListWithSize(promptStr string, options []string, size int) (string, error) {
	prompt := promptui.Select{
		Stdout: ux.Logger,
		Label:  promptStr,
		Items:  options,
		Size:   size,
	}
	_, listDecision, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return listDecision, nil
}

func (*realPrompter) CaptureEmail(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
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
		Stdout: ux.Logger,
		Label:  promptStr,
	}

	str, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureURL(promptStr string, validateConnection bool) (string, error) {
	for {
		prompt := promptui.Prompt{
			Stdout:   ux.Logger,
			Label:    promptStr,
			Validate: ValidateURLFormat,
		}
		str, err := prompt.Run()
		if err != nil {
			return "", err
		}
		if !validateConnection {
			return str, nil
		}
		if err := ValidateURL(str); err == nil {
			return str, nil
		}
		ux.Logger.PrintToUser("Invalid URL: %s", err)
	}
}

func (*realPrompter) CaptureRepoBranch(promptStr string, repo string) (string, error) {
	for {
		var err error
		prompt := promptui.Prompt{
			Stdout:   ux.Logger,
			Label:    promptStr,
			Validate: validateNonEmpty,
		}
		str, err := prompt.Run()
		if err != nil {
			return "", err
		}
		if err = ValidateRepoBranch(repo, str); err == nil {
			return str, nil
		}
		ux.Logger.PrintToUser("Invalid Repo Branch: %s", err)
	}
}

func (*realPrompter) CaptureRepoFile(promptStr string, repo string, branch string) (string, error) {
	for {
		var err error
		prompt := promptui.Prompt{
			Stdout:   ux.Logger,
			Label:    promptStr,
			Validate: validateNonEmpty,
		}
		str, err := prompt.Run()
		if err != nil {
			return "", err
		}
		if err = ValidateRepoFile(repo, branch, str); err == nil {
			return str, nil
		}
		ux.Logger.PrintToUser("Invalid Repo File: %s", err)
	}
}

func (*realPrompter) CaptureString(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: validateNonEmpty,
	}

	str, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureValidatedString(promptStr string, validator func(string) error) (string, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: validator,
	}

	str, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureGitURL(promptStr string) (*url.URL, error) {
	prompt := promptui.Prompt{
		Stdout:   ux.Logger,
		Label:    promptStr,
		Validate: ValidateURLFormat,
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
		Stdout: ux.Logger,
		Label:  promptStr,
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
		Stdout: ux.Logger,
		Label:  promptStr,
		Items:  options,
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
		Stdout: ux.Logger,
		Label:  promptStr,
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
		fmt.Sprintf("Which key should be used %s?", goal),
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
func GetSubnetAuthKeys(prompt Prompter, walletKeys []string, controlKeys []string, threshold uint32) ([]string, error) {
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
		subnetAuthKey, err := prompt.CaptureList(
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

func GetKeyOrLedger(prompt Prompter, goal string, keyDir string, includeEwoq bool) (bool, string, error) {
	useStoredKey, err := prompt.ChooseKeyOrLedger(goal)
	if err != nil {
		return false, "", err
	}
	if !useStoredKey {
		return true, "", nil
	}
	keyName, err := CaptureKeyName(prompt, goal, keyDir, includeEwoq)
	if err != nil {
		if errors.Is(err, errNoKeys) {
			ux.Logger.PrintToUser("No private keys have been found. Create a new one with `avalanche key create`")
		}
		return false, "", err
	}
	return false, keyName, nil
}

func CaptureKeyName(prompt Prompter, goal string, keyDir string, includeEwoq bool) (string, error) {
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
	keyName, err := prompt.CaptureListWithSize(fmt.Sprintf("Which stored key should be used %s?", goal), keyNames, size)
	if err != nil {
		return "", err
	}
	return keyName, nil
}

func CaptureBoolFlag(
	prompt Prompter,
	cmd *cobra.Command,
	flagName string,
	flagValue bool,
	promptMsg string,
) (bool, error) {
	if flagValue {
		return true, nil
	}
	if flag := cmd.Flags().Lookup(flagName); flag == nil || !flag.Changed {
		return prompt.CaptureYesNo(promptMsg)
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

// PromptLongString reads a long string from the user input.
func PromptLongString(msg string, args ...interface{}) (string, error) {
	_, _ = ux.Logger.Write([]byte(fmt.Sprintf(msg, args...)))
	reader := bufio.NewReader(os.Stdin)
	longString, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	// Remove newline character at the end
	longString = strings.TrimSuffix(longString, "\n")
	ux.Logger.Info(longString)
	return longString, nil
}
