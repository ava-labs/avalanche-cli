// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	errInvalidPrecompiles       = errors.New("invalid precompiles")
	errInvalidPrecompileContent = errors.New("invalid precompile content")
	errNoBlockTimestamp         = errors.New("no blockTimestamp value set")
	errBlockTimestampInvalid    = errors.New("blockTimestamp is invalid")
	errBlockTimestampInthePast  = errors.New("blockTimestamp is in the past")
	errNoPrecompiles            = errors.New("no precompiles present")
	errEmptyPrecompile          = errors.New("the precompile has no content")
	errNoUpcomingUpgrades       = errors.New("no valid upcoming activation timestamp found")
)

func validateUpgradeBytes(file []byte) error {
	precomps, err := getAllPrecomps(file)
	if err != nil {
		return err
	}

	allTimestamps, err := getAllTimestamps(precomps)
	if err != nil {
		return err
	}

	for _, ts := range allTimestamps {
		if time.Unix(ts, 0).Before(time.Now()) {
			return errBlockTimestampInthePast
		}
	}

	// TODO what other validation do we want to do?
	return nil
}

func getAllTimestamps(precomps []PrecompileContent) ([]int64, error) {
	allTimestamps := make([]int64, len(precomps))

	var (
		ok              bool
		blockTimestamp  float64 // json unmarshalling of numbers is always float64...
		blockTimestampV interface{}
	)

	for i, pre := range precomps {
		if blockTimestampV, ok = pre[blockTimestampKey]; !ok {
			return nil, errNoBlockTimestamp
		}
		if blockTimestamp, ok = blockTimestampV.(float64); !ok {
			return nil, errBlockTimestampInvalid
		}
		allTimestamps[i] = int64(blockTimestamp)
	}
	return allTimestamps, nil
}

func getEarliestTimestamp(precomps []PrecompileContent) (int64, error) {
	allTimestamps, err := getAllTimestamps(precomps)
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

func getAllPrecomps(file []byte) ([]PrecompileContent, error) {
	var (
		ok                bool
		precompiles       Precompiles
		precompileContent PrecompileContent
	)

	if err := json.Unmarshal(file, &precompiles); err != nil {
		return nil, fmt.Errorf("failed parsing JSON - %s: %w", err.Error(), errInvalidPrecompiles)
	}

	if precompiles.PrecompileUpgrades == nil || len(precompiles.PrecompileUpgrades) == 0 {
		return nil, errNoPrecompiles
	}

	allPrecomps := make([]PrecompileContent, 0, len(precompiles.PrecompileUpgrades))

	for name, precomp := range precompiles.PrecompileUpgrades {
		if precompileContent, ok = precomp.(map[string]interface{}); !ok {
			return nil, fmt.Errorf("failed unpacking JSON for the %s precompile: %w", name, errInvalidPrecompileContent)
		}
		if precompileContent == nil {
			return nil, errEmptyPrecompile
		}
		allPrecomps = append(allPrecomps, precompileContent)
	}
	return allPrecomps, nil
}
