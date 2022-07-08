// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
	"github.com/manifoldco/promptui"
)

const (
	Yes = "Yes"
	No  = "No"
)

type (
	PromptCreateFunc func(string) PromptRunner
	SelectCreateFunc func(string, interface{}) SelectRunner
)

// PromptRunner is an interface which allows to mock the promptui stuff
type PromptRunner interface {
	Run() (string, error)
	SetValidation(func(string) error)
}

type SelectRunner interface {
	Run() (int, string, error)
}

type Prompter struct {
	prompt promptui.Prompt
}

type Selector struct {
	selector promptui.Select
}

func NewPrompter(str string) PromptRunner {
	return &Prompter{
		prompt: promptui.Prompt{
			Label: str,
		},
	}
}

func (p *Prompter) SetValidation(valFunc func(string) error) {
	p.prompt.Validate = valFunc
}

func (p *Prompter) Run() (string, error) {
	return p.prompt.Run()
}

func NewSelector(str string, items interface{}) SelectRunner {
	return &Selector{
		selector: promptui.Select{
			Label: str,
			Items: items,
		},
	}
}

func (s *Selector) Run() (int, string, error) {
	return s.selector.Run()
}

func CaptureDuration(prompt PromptRunner) (time.Duration, error) {
	prompt.SetValidation(validateStakingDuration)

	durationStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(durationStr)
}

func CaptureDate(prompt PromptRunner) (time.Time, error) {
	prompt.SetValidation(validateTime)

	timeStr, err := prompt.Run()
	if err != nil {
		return time.Time{}, err
	}

	return time.Parse(constants.TimeParseLayout, timeStr)
}

func CaptureNodeID(prompt PromptRunner) (ids.NodeID, error) {
	prompt.SetValidation(validateNodeID)

	nodeIDStr, err := prompt.Run()
	if err != nil {
		return ids.EmptyNodeID, err
	}
	return ids.NodeIDFromString(nodeIDStr)
}

func CaptureWeight(prompt PromptRunner) (uint64, error) {
	prompt.SetValidation(validateWeight)

	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(amountStr, 10, 64)
}

func CaptureUint64(prompt PromptRunner) (uint64, error) {
	prompt.SetValidation(validateBiggerThanZero)

	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(amountStr, 10, 64)
}

func CapturePositiveBigInt(prompt PromptRunner) (*big.Int, error) {
	prompt.SetValidation(validatePositiveBigInt)

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

func CapturePChainAddress(prompt PromptRunner) (string, error) {
	prompt.SetValidation(validatePChainAddress)

	addressStr, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return addressStr, nil
}

func CaptureAddress(prompt PromptRunner) (common.Address, error) {
	prompt.SetValidation(validateAddress)

	addressStr, err := prompt.Run()
	if err != nil {
		return common.Address{}, err
	}

	addressHex := common.HexToAddress(addressStr)
	return addressHex, nil
}

func CaptureExistingFilepath(prompt PromptRunner) (string, error) {
	prompt.SetValidation(validateExistingFilepath)

	pathStr, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return pathStr, nil
}

func yesNoBase(prompt SelectRunner) (bool, error) {
	_, decision, err := prompt.Run()
	if err != nil {
		return false, err
	}
	return decision == Yes, nil
}

func CaptureYesNo(f SelectCreateFunc, promptStr string) (bool, error) {
	return yesNoBase(f(promptStr, []string{Yes, No}))
}

func CaptureNoYes(f SelectCreateFunc, promptStr string) (bool, error) {
	return yesNoBase(f(promptStr, []string{No, Yes}))
}

func CaptureList(prompt SelectRunner) (string, error) {
	_, listDecision, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return listDecision, nil
}

func CaptureString(prompt PromptRunner) (string, error) {
	prompt.SetValidation(
		func(input string) error {
			if input == "" {
				return errors.New("string cannot be empty")
			}
			return nil
		},
	)

	str, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return str, nil
}

func CaptureIndex(prompt SelectRunner) (int, error) {
	listIndex, _, err := prompt.Run()
	if err != nil {
		return 0, err
	}
	return listIndex, nil
}
