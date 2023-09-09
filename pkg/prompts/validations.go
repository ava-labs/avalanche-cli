// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package prompts

import (
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanchego/genesis"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/ids"
	avago_constants "github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ethereum/go-ethereum/common"
)

func validateEmail(input string) error {
	_, err := mail.ParseAddress(input)
	return err
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

func validateMainnetStakingDuration(input string) error {
	d, err := time.ParseDuration(input)
	if err != nil {
		return err
	}
	if d > genesis.MainnetParams.MaxStakeDuration {
		return fmt.Errorf("exceeds maximum staking duration of %s", ux.FormatDuration(genesis.MainnetParams.MaxStakeDuration))
	}
	if d < genesis.MainnetParams.MinStakeDuration {
		return fmt.Errorf("below the minimum staking duration of %s", ux.FormatDuration(genesis.MainnetParams.MinStakeDuration))
	}
	return nil
}

func validateFujiStakingDuration(input string) error {
	d, err := time.ParseDuration(input)
	if err != nil {
		return err
	}
	if d > genesis.FujiParams.MaxStakeDuration {
		return fmt.Errorf("exceeds maximum staking duration of %s", ux.FormatDuration(genesis.FujiParams.MaxStakeDuration))
	}
	if d < genesis.FujiParams.MinStakeDuration {
		return fmt.Errorf("below the minimum staking duration of %s", ux.FormatDuration(genesis.FujiParams.MinStakeDuration))
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
	if val < constants.MinStakeWeight {
		return errors.New("the weight must be an integer between 1 and 100")
	}
	return nil
}

func validateBiggerThanZero(input string) error {
	val, err := strconv.ParseUint(input, 0, 64)
	if err != nil {
		return err
	}
	if val == 0 {
		return errors.New("the value must be bigger than zero")
	}
	return nil
}

func validateURLFormat(input string) error {
	_, err := url.ParseRequestURI(input)
	if err != nil {
		return err
	}
	return nil
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

func validateID(input string) error {
	_, err := ids.FromString(input)
	return err
}

func validateNewFilepath(input string) error {
	if _, err := os.Stat(input); err != nil && os.IsNotExist(err) {
		return nil
	}
	return errors.New("file already exists")
}

func validateNonEmpty(input string) error {
	if input == "" {
		return errors.New("string cannot be empty")
	}
	return nil
}

func RequestURL(url string) (*http.Response, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for url %s: %w", url, err)
	}
	token := os.Getenv(constants.GithubAPITokenEnvVarName)
	if token != "" {
		// avoid rate limitation issues at CI
		request.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status code: %d", resp.StatusCode)
	}
	return resp, nil
}

func ValidateURL(url string) error {
	if err := validateURLFormat(url); err != nil {
		return err
	}
	resp, err := RequestURL(url)
	if err != nil {
		return err
	}
	// will just ignore this error, url is already validated
	_ = resp.Body.Close()
	return nil
}

func ValidateRepoBranch(repo string, branch string) error {
	url := repo + "/tree/" + branch
	return ValidateURL(url)
}

func ValidateRepoFile(repo string, branch string, file string) error {
	url := repo + "/blob/" + branch + "/" + file
	return ValidateURL(url)
}
