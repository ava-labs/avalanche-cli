package cmd

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/manifoldco/promptui"
)

// func validateInt(input string) error {
// 	_, err := strconv.ParseInt(input, 10, 64)
// 	if err != nil {
// 		return errors.New("Invalid number")
// 	}
// 	return nil
// }

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

func capturePositiveBigInt(promptStr string) (*big.Int, error) {
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

func captureAddress(promptStr string) (common.Address, error) {
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

func captureYesNo(promptStr string) (bool, error) {
	const yes = "Yes"
	const no = "No"
	prompt := promptui.Select{
		Label: promptStr,
		Items: []string{yes, no},
	}

	_, decision, err := prompt.Run()
	if err != nil {
		return false, err
	}
	return decision == yes, nil
}

func captureList(promptStr string, options []string) (string, error) {
	prompt := promptui.Select{
		Label: "Configure contract deployment allow list:",
		Items: options,
	}

	_, listDecision, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return listDecision, nil
}
