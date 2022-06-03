// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"errors"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/manifoldco/promptui"
)

const yes = "Yes"
const no = "No"

func validatePositiveBigInt(input string) error {
	n := new(big.Int)
	n, ok := n.SetString(input, 10)
	if !ok {
		return errors.New("Invalid number")
	}
	if n.Cmp(big.NewInt(0)) == -1 {
		return errors.New("Invalid number")
	}
	return nil
}

func validateAddress(input string) error {
	if !common.IsHexAddress(input) {
		return errors.New("Invalid address")
	}
	return nil
}

func validateExistingFilepath(input string) error {
	if fileInfo, err := os.Stat(input); err == nil && !fileInfo.IsDir() {
		return nil
	}
	return errors.New("File doesn't exist")
}

func CapturePositiveBigInt(promptStr string) (*big.Int, error) {
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

func CaptureAddress(promptStr string) (common.Address, error) {
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

func CaptureExistingFilepath(promptStr string) (string, error) {
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

func yesNoBase(promptStr string, orderedOptions []string) (bool, error) {
	prompt := promptui.Select{
		Label: promptStr,
		Items: orderedOptions,
	}

	_, decision, err := prompt.Run()
	if err != nil {
		return false, err
	}
	return decision == yes, nil
}

func CaptureYesNo(promptStr string) (bool, error) {
	return yesNoBase(promptStr, []string{yes, no})
}

func CaptureNoYes(promptStr string) (bool, error) {
	return yesNoBase(promptStr, []string{no, yes})
}

func CaptureList(promptStr string, options []string) (string, error) {
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

func CaptureString(promptStr string) (string, error) {
	prompt := promptui.Prompt{
		Label: promptStr,
		Validate: func(input string) error {
			if input == "" {
				return errors.New("String cannot be empty")
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

func CaptureIndex(promptStr string, options []common.Address) (int, error) {
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
