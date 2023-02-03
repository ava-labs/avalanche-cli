// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ava-labs/subnet-evm/params"
)

var (
	errInvalidPrecompiles      = errors.New("invalid precompiles")
	errNoBlockTimestamp        = errors.New("no blockTimestamp value set")
	errBlockTimestampInvalid   = errors.New("blockTimestamp is invalid")
	errBlockTimestampInthePast = errors.New("blockTimestamp is in the past")
	errNoPrecompiles           = errors.New("no precompiles present")
	errEmptyPrecompile         = errors.New("the precompile has no content")
	errNoUpcomingUpgrades      = errors.New("no valid upcoming activation timestamp found")
)

func validateUpgradeBytes(file []byte) ([]params.PrecompileUpgrade, error) {
	upgrades, err := getAllUpgrades(file)
	if err != nil {
		return nil, err
	}

	allTimestamps, err := getAllTimestamps(upgrades)
	if err != nil {
		return nil, err
	}

	for _, ts := range allTimestamps {
		if time.Unix(ts, 0).Before(time.Now()) {
			return nil, errBlockTimestampInthePast
		}
	}

	return upgrades, nil
}

func getAllTimestamps(upgrades []params.PrecompileUpgrade) ([]int64, error) {
	var allTimestamps []int64

	if len(upgrades) == 0 {
		return nil, errNoBlockTimestamp
	}
	for _, upgrade := range upgrades {
		if upgrade.ContractDeployerAllowListConfig != nil {
			ts, err := validateTimestamp(upgrade.ContractDeployerAllowListConfig.BlockTimestamp)
			if err != nil {
				return nil, err
			}
			allTimestamps = append(allTimestamps, ts)
		}
		if upgrade.FeeManagerConfig != nil {
			ts, err := validateTimestamp(upgrade.FeeManagerConfig.BlockTimestamp)
			if err != nil {
				return nil, err
			}
			allTimestamps = append(allTimestamps, ts)
		}
		if upgrade.ContractNativeMinterConfig != nil {
			ts, err := validateTimestamp(upgrade.ContractNativeMinterConfig.BlockTimestamp)
			if err != nil {
				return nil, err
			}
			allTimestamps = append(allTimestamps, ts)
		}
		if upgrade.TxAllowListConfig != nil {
			ts, err := validateTimestamp(upgrade.TxAllowListConfig.BlockTimestamp)
			if err != nil {
				return nil, err
			}
			allTimestamps = append(allTimestamps, ts)
		}
	}
	if len(allTimestamps) == 0 {
		return nil, errNoBlockTimestamp
	}
	return allTimestamps, nil
}

func validateTimestamp(ts *big.Int) (int64, error) {
	if ts == nil {
		return 0, errNoBlockTimestamp
	}
	if !ts.IsInt64() {
		return 0, errBlockTimestampInvalid
	}
	val := ts.Int64()
	if val == int64(0) {
		return 0, errBlockTimestampInvalid
	}
	return val, nil
}

func getEarliestTimestamp(upgrades []params.PrecompileUpgrade) (int64, error) {
	allTimestamps, err := getAllTimestamps(upgrades)
	if err != nil {
		return 0, err
	}

	earliest := int64(math.MaxInt64)

	for _, ts := range allTimestamps {
		// we may also not necessarily need to check
		// if after now, but to know if something is upcoming,
		// seems appropriate
		if ts < earliest && time.Unix(ts, 0).After(time.Now()) {
			earliest = ts
		}
	}

	// this should not happen as we have timestamp validation
	// but might be required if called in a different context
	if earliest == math.MaxInt64 {
		return earliest, errNoUpcomingUpgrades
	}

	return earliest, nil
}

func getAllUpgrades(file []byte) ([]params.PrecompileUpgrade, error) {
	var precompiles params.UpgradeConfig

	if err := json.Unmarshal(file, &precompiles); err != nil {
		return nil, fmt.Errorf("failed parsing JSON - %s: %w", err.Error(), errInvalidPrecompiles)
	}

	if len(precompiles.PrecompileUpgrades) == 0 {
		return nil, errNoPrecompiles
	}

	for _, upgrade := range precompiles.PrecompileUpgrades {
		if upgrade.ContractDeployerAllowListConfig == nil &&
			upgrade.ContractNativeMinterConfig == nil &&
			upgrade.FeeManagerConfig == nil &&
			upgrade.TxAllowListConfig == nil {
			return nil, errEmptyPrecompile
		}
	}
	return precompiles.PrecompileUpgrades, nil
}
