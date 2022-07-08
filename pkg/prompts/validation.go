package prompts

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ethereum/go-ethereum/common"
)

var (
	errInvalidNumber             = errors.New("invalid number")
	errExceedsMaxStakingDuration = errors.New("exceeds maximum staking duration of 1 year")
	errBelowMinStakingDuration   = errors.New("below the minimum staking duration of two weeks")
	errInvalidAddress            = errors.New("invalid address")
	errFileNoExists              = errors.New("file doesn't exist")
	errInvalidWeight             = errors.New("the weight must be between 1 and 100")
	errMustBeBiggerThanZero      = errors.New("the value must be bigger than zero")
	errTooEarly                  = fmt.Errorf("time should be at least start from now + %s", constants.StakingStartLeeTime)
)

func validatePositiveBigInt(input string) error {
	n := new(big.Int)
	n, ok := n.SetString(input, 10)
	if !ok {
		return errInvalidNumber
	}
	if n.Cmp(big.NewInt(0)) == -1 {
		return errInvalidNumber
	}
	return nil
}

func validateStakingDuration(input string) error {
	d, err := time.ParseDuration(input)
	if err != nil {
		return err
	}
	if d > constants.MaxStakeDuration {
		return errExceedsMaxStakingDuration
	}
	if d < constants.MinStakeDuration {
		return errBelowMinStakingDuration
	}
	return nil
}

func validateTime(input string) error {
	t, err := time.Parse(constants.TimeParseLayout, input)
	if err != nil {
		return err
	}
	if t.Before(time.Now().Add(constants.StakingStartLeeTime)) {
		return errTooEarly
	}
	return err
}

func validateNodeID(input string) error {
	_, err := ids.NodeIDFromString(input)
	return err
}

func validateAddress(input string) error {
	if !common.IsHexAddress(input) {
		return errInvalidAddress
	}
	return nil
}

func validateExistingFilepath(input string) error {
	if fileInfo, err := os.Stat(input); err == nil && !fileInfo.IsDir() {
		return nil
	}
	return errFileNoExists
}

func validateWeight(input string) error {
	val, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		return err
	}
	if val < 1 || val > 100 {
		return errInvalidWeight
	}
	return nil
}

func validateBiggerThanZero(input string) error {
	val, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		return err
	}
	if val == 0 {
		return errMustBeBiggerThanZero
	}
	return nil
}

func validatePChainAddress(input string) error {
	_, _, _, err := address.Parse(input)

	return err
}
