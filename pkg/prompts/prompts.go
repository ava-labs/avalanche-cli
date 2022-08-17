// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ethereum/go-ethereum/common"
	"github.com/manifoldco/promptui"
)

const (
	Yes = "Yes"
	No  = "No"

	Add      = "Add"
	Del      = "Del"
	Preview  = "Preview"
	MoreInfo = "More Info"
	Done     = "Done"
	Cancel   = "Cancel"
)

type Prompter interface {
	CapturePositiveBigInt(promptStr string) (*big.Int, error)
	CaptureAddress(promptStr string, arg any) (any, error)
	CaptureExistingFilepath(promptStr string) (string, error)
	CaptureYesNo(promptStr string) (bool, error)
	CaptureNoYes(promptStr string) (bool, error)
	CaptureList(promptStr string, options []string) (string, error)
	CaptureString(promptStr string) (string, error)
	CaptureIndex(promptStr string, options []interface{}) (int, error)
	CaptureDuration(promptStr string) (time.Duration, error)
	CaptureDate(promptStr string) (time.Time, error)
	CaptureNodeID(promptStr string) (ids.NodeID, error)
	CaptureWeight(promptStr string) (uint64, error)
	CaptureUint64(promptStr string) (uint64, error)
	CapturePChainAddress(promptStr string, network any) (any, error)
	CaptureListDecision(
		prompter Prompter, // we need this in order to be able to run mock tests
		prompt string,
		capture func(prompt string, args any) (any, error),
		capturePrompt string,
		label string,
		info string,
		arg any,
	) ([]any, bool, error)
}

type realPrompter struct{}

// NewProcessChecker creates a new process checker which can respond if the server is running
func NewPrompter() Prompter {
	return &realPrompter{}
}

func validatePositiveBigInt(input string) error {
	n := new(big.Int)
	n, ok := n.SetString(input, 10)
	if !ok {
		return errors.New("invalid number")
	}
	if n.Cmp(big.NewInt(0)) == -1 {
		return errors.New("invalid number")
	}
	return nil
}

func validateStakingDuration(input string) error {
	d, err := time.ParseDuration(input)
	if err != nil {
		return err
	}
	if d > constants.MaxStakeDuration {
		return fmt.Errorf("exceeds maximum staking duration of %s", ux.FormatDuration(constants.MaxStakeDuration))
	}
	if d < constants.MinStakeDuration {
		return fmt.Errorf("below the minimum staking duration of %s", ux.FormatDuration(constants.MinStakeDuration))
	}
	return nil
}

func validateTime(input string) error {
	t, err := time.Parse(constants.TimeParseLayout, input)
	if err != nil {
		return err
	}
	if t.Before(time.Now().Add(constants.StakingStartLeadTime)) {
		return fmt.Errorf("time should be at least start from now + %s", constants.StakingStartLeadTime)
	}
	return err
}

func validateNodeID(input string) error {
	_, err := ids.NodeIDFromString(input)
	return err
}

func validateAddress(input string) error {
	if !common.IsHexAddress(input) {
		return errors.New("invalid address")
	}
	return nil
}

func validateExistingFilepath(input string) error {
	if fileInfo, err := os.Stat(input); err == nil && !fileInfo.IsDir() {
		return nil
	}
	return errors.New("file doesn't exist")
}

func validateWeight(input string) error {
	val, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		return err
	}
	if val < constants.MinStakeWeight || val > constants.MaxStakeWeight {
		return errors.New("the weight must be an integer between 1 and 100")
	}
	return nil
}

func validateBiggerThanZero(input string) error {
	val, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		return err
	}
	if val == 0 {
		return errors.New("the value must be bigger than zero")
	}
	return nil
}

func (r *realPrompter) CaptureListDecision(
	prompter Prompter, // we need this in order to be able to run mock tests
	prompt string,
	capture func(prompt string, args any) (any, error),
	capturePrompt string,
	label string,
	info string,
	arg any,
) ([]any, bool, error) {
	finalList := []any{}
	for {
		listDecision, err := prompter.CaptureList(
			prompt, []string{Add, Del, Preview, MoreInfo, Done, Cancel},
		)
		if err != nil {
			return nil, false, err
		}
		switch listDecision {
		case Add:
			elem, err := capture(
				capturePrompt,
				arg,
			)
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
				fmt.Println("no " + label + " added yet")
				continue
			}
			index, err := prompter.CaptureIndex("Choose element to remove:", finalList)
			if err != nil {
				return nil, false, err
			}
			finalList = append(finalList[:index], finalList[index+1:]...)
		case Preview:
			for i, k := range finalList {
				fmt.Printf("%d. %s\n", i, k)
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
		Label:    promptStr,
		Validate: validateStakingDuration,
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

func (*realPrompter) CaptureUint64(promptStr string) (uint64, error) {
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: validateBiggerThanZero,
	}

	amountStr, err := prompt.Run()
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(amountStr, 10, 64)
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

func validatePChainAddress(input string) (string, error) {
	chainID, hrp, _, err := address.Parse(input)
	if err != nil {
		return "", err
	}

	if chainID != "P" {
		return "", errors.New("this is not a PChain address")
	}
	return hrp, nil
}

func validatePChainFujiAddress(input string) error {
	hrp, err := validatePChainAddress(input)
	if err != nil {
		return err
	}
	if hrp != avago_constants.FujiHRP {
		return errors.New("this is not a fuji address")
	}
	return nil
}

func validatePChainMainAddress(input string) error {
	hrp, err := validatePChainAddress(input)
	if err != nil {
		return err
	}
	if hrp != avago_constants.MainnetHRP {
		return errors.New("this is not a mainnet address")
	}
	return nil
}

func validatePChainLocalAddress(input string) error {
	hrp, err := validatePChainAddress(input)
	if err != nil {
		return err
	}
	// ANR uses the `custom` HRP for local networks,
	// but the `local` HRP also exists...
	if hrp != avago_constants.LocalHRP && hrp != avago_constants.FallbackHRP {
		return errors.New("this is not a local nor custom address")
	}
	return nil
}

func getPChainValidationFunc(network models.Network) func(string) error {
	switch network {
	case models.Fuji:
		return validatePChainFujiAddress
	case models.Mainnet:
		return validatePChainMainAddress
	case models.Local:
		return validatePChainLocalAddress
	default:
		return func(string) error {
			return errors.New("unsupported network")
		}
	}
}

func (*realPrompter) CapturePChainAddress(promptStr string, net any) (any, error) {
	network := net.(models.Network)
	prompt := promptui.Prompt{
		Label:    promptStr,
		Validate: getPChainValidationFunc(network),
	}

	return prompt.Run()
}

func (*realPrompter) CaptureAddress(promptStr string, arg any) (any, error) {
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

func (*realPrompter) CaptureIndex(promptStr string, options []interface{}) (int, error) {
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

func contains(list []any, element any) bool {
	for _, val := range list {
		if val == element {
			return true
		}
	}
	return false
}
