// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts/comparator"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	sdkutils "github.com/ava-labs/avalanche-tooling-sdk-go/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/libevm/common"

	"github.com/manifoldco/promptui"
	"golang.org/x/mod/semver"
)

const (
	Yes = "Yes"
	No  = "No"
)

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
	CapturePositiveInt(promptStr string, comparators []comparator.Comparator) (int, error)
	CaptureInt(promptStr string, validator func(int) error) (int, error)
	CaptureUint8(promptStr string) (uint8, error)
	CaptureUint16(promptStr string) (uint16, error)
	CaptureUint32(promptStr string) (uint32, error)
	CaptureUint64(promptStr string) (uint64, error)
	CaptureFloat(promptStr string, validator func(float64) error) (float64, error)
	CaptureUint64Compare(promptStr string, comparators []comparator.Comparator) (uint64, error)
	CapturePChainAddress(promptStr string, network models.Network) (string, error)
	CaptureXChainAddress(promptStr string, network models.Network) (string, error)
	CaptureFutureDate(promptStr string, minDate time.Time) (time.Time, error)
	ChooseKeyOrLedger(goal string) (bool, error)
}

type realPrompter struct{}

// Global variable that can be replaced during testing
var promptUIRunner = func(prompt promptui.Prompt) (string, error) {
	return prompt.Run()
}

// Global variable for Select operations that can be replaced during testing
var promptUISelectRunner = func(prompt promptui.Select) (int, string, error) {
	return prompt.Run()
}

// Global variable for ReadLongString operations that can be replaced during testing
var utilsReadLongString = utils.ReadLongString

func NewPrompter() Prompter {
	return &realPrompter{}
}

func (*realPrompter) CaptureDuration(promptStr string) (time.Duration, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateDuration,
	}

	durationStr, err := promptUIRunner(prompt)
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(durationStr)
}

func (*realPrompter) CaptureFujiDuration(promptStr string) (time.Duration, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateFujiStakingDuration,
	}

	durationStr, err := promptUIRunner(prompt)
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

	durationStr, err := promptUIRunner(prompt)
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(durationStr)
}

func (*realPrompter) CaptureMainnetL1StakingDuration(promptStr string) (time.Duration, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateMainnetL1StakingDuration,
	}

	durationStr, err := promptUIRunner(prompt)
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

	timeStr, err := promptUIRunner(prompt)
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

	idStr, err := promptUIRunner(prompt)
	if err != nil {
		return ids.Empty, err
	}
	return ids.FromString(idStr)
}

func (*realPrompter) CaptureNodeID(promptStr string) (ids.NodeID, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: ValidateNodeID,
	}

	nodeIDStr, err := promptUIRunner(prompt)
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
		Label:    promptStr,
		Validate: validateValidatorBalanceFunc(availableBalance, minBalance),
	}
	amountStr, err := promptUIRunner(prompt)
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
		Label:    promptStr,
		Validate: validateWeightFunc(validator),
	}

	amountStr, err := promptUIRunner(prompt)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(amountStr, 10, 64)
}

func (*realPrompter) CaptureInt(promptStr string, validator func(int) error) (int, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(input string) error {
			val, err := strconv.Atoi(input)
			if err != nil {
				return err
			}
			return validator(val)
		},
	}
	input, err := promptUIRunner(prompt)
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
		Label: promptStr,
		Validate: func(input string) error {
			_, err := strconv.ParseUint(input, 0, 8)
			if err != nil {
				return err
			}
			return nil
		},
	}
	input, err := promptUIRunner(prompt)
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
		Label: promptStr,
		Validate: func(input string) error {
			_, err := strconv.ParseUint(input, 0, 16)
			if err != nil {
				return err
			}
			return nil
		},
	}
	input, err := promptUIRunner(prompt)
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
		Label: promptStr,
		Validate: func(input string) error {
			_, err := strconv.ParseUint(input, 0, 32)
			if err != nil {
				return err
			}
			return nil
		},
	}
	input, err := promptUIRunner(prompt)
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

	amountStr, err := promptUIRunner(prompt)
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

	amountStr, err := promptUIRunner(prompt)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(amountStr, 64)
}

func (*realPrompter) CapturePositiveInt(promptStr string, comparators []comparator.Comparator) (int, error) {
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

	amountStr, err := promptUIRunner(prompt)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(amountStr)
}

func (*realPrompter) CaptureUint64Compare(promptStr string, comparators []comparator.Comparator) (uint64, error) {
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

	amountStr, err := promptUIRunner(prompt)
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

	amountStr, err := promptUIRunner(prompt)
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

	return promptUIRunner(prompt)
}

func (*realPrompter) CaptureXChainAddress(promptStr string, network models.Network) (string, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: getXChainValidationFunc(network),
	}

	return promptUIRunner(prompt)
}

func (*realPrompter) CaptureAddress(promptStr string) (common.Address, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: ValidateAddress,
	}

	addressStr, err := promptUIRunner(prompt)
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
		addressesStr, err = utilsReadLongString(promptui.IconGood + " " + promptStr + " ")
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
		Label:    promptStr,
		Validate: validateExistingFilepath,
	}

	pathStr, err := promptUIRunner(prompt)
	if err != nil {
		return "", err
	}
	pathStr = utils.ExpandHome(pathStr)

	return pathStr, nil
}

func (*realPrompter) CaptureNewFilepath(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateNewFilepath,
	}

	pathStr, err := promptUIRunner(prompt)
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

	_, decision, err := promptUISelectRunner(prompt)
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
	_, listDecision, err := promptUISelectRunner(prompt)
	if err != nil {
		return "", err
	}
	return listDecision, nil
}

func (*realPrompter) CaptureListWithSize(promptStr string, options []string, size int) (string, error) {
	prompt := promptui.Select{
		Label: promptStr,
		Items: options,
		Size:  size,
	}
	_, listDecision, err := promptUISelectRunner(prompt)
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

	str, err := promptUIRunner(prompt)
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureStringAllowEmpty(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
	}

	str, err := promptUIRunner(prompt)
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureURL(promptStr string, validateConnection bool) (string, error) {
	for {
		prompt := promptui.Prompt{
			Label:    promptStr,
			Validate: ValidateURLFormat,
		}
		str, err := promptUIRunner(prompt)
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
			Label:    promptStr,
			Validate: validateNonEmpty,
		}
		str, err := promptUIRunner(prompt)
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
			Label:    promptStr,
			Validate: validateNonEmpty,
		}
		str, err := promptUIRunner(prompt)
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
		Label:    promptStr,
		Validate: validateNonEmpty,
	}

	str, err := promptUIRunner(prompt)
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureValidatedString(promptStr string, validator func(string) error) (string, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validator,
	}

	str, err := promptUIRunner(prompt)
	if err != nil {
		return "", err
	}

	return str, nil
}

func (*realPrompter) CaptureGitURL(promptStr string) (*url.URL, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: ValidateURLFormat,
	}

	str, err := promptUIRunner(prompt)
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

	str, err := promptUIRunner(prompt)
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

	listIndex, _, err := promptUISelectRunner(prompt)
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

	timestampStr, err := promptUIRunner(prompt)
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
