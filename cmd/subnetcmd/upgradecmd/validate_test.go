// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package upgradecmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEarliestTimestamp(t *testing.T) {
	type testRun struct {
		name         string
		upgradesFile []byte
		earliest     int64
		expectedErr  error
	}

	targetEarliest := time.Now().Add(1 * time.Minute).Unix()
	tests := []testRun{
		{
			name: "only one",
			upgradesFile: []byte(
				fmt.Sprintf(`
{"precompileUpgrades":[
{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}
]}`,
					targetEarliest,
				)),
			earliest:    targetEarliest,
			expectedErr: nil,
		},
		{
			name: "there are two",
			upgradesFile: []byte(
				fmt.Sprintf(`
{"precompileUpgrades":[
{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"contractNativeMinterConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}
]}`,
					targetEarliest,
					time.Now().Add(1*time.Minute).Add(2*time.Second).Unix(),
				)),
			earliest:    targetEarliest,
			expectedErr: nil,
		},
		{
			name: "three with second earliest",
			upgradesFile: []byte(
				fmt.Sprintf(`
{"precompileUpgrades":[
{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"contractNativeMinterConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"txAllowListConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}
]}`,
					time.Now().Add(1*time.Minute).Add(2*time.Second).Unix(),
					targetEarliest,
					time.Now().Add(1*time.Minute).Add(4*time.Second).Unix(),
				)),
			earliest:    targetEarliest,
			expectedErr: nil,
		},
		{
			name: "three with third earliest",
			upgradesFile: []byte(
				fmt.Sprintf(`
{"precompileUpgrades":[
{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"txAllowListConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"contractNativeMinterConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}
]}`,
					time.Now().Add(1*time.Minute).Add(2*time.Second).Unix(),
					time.Now().Add(1*time.Minute).Add(4*time.Second).Unix(),
					targetEarliest,
				)),
			earliest:    targetEarliest,
			expectedErr: nil,
		},
		{
			name: "no upcoming",
			upgradesFile: []byte(
				fmt.Sprintf(`
{"precompileUpgrades":[
{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"txAllowListConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"contractNativeMinterConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}
]}`,
					time.Now().Add(10*time.Millisecond).Unix(),
					time.Now().Add(15*time.Millisecond).Unix(),
					time.Now().Add(20*time.Millisecond).Unix(),
				)),
			earliest:    targetEarliest,
			expectedErr: errNoUpcomingUpgrades,
		},
	}

	require := require.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrades, err := getAllUpgrades(tt.upgradesFile)
			require.NoError(err)
			earliest, err := getEarliestUpcomingTimestamp(upgrades)
			if tt.expectedErr != nil {
				// give some time so timestamps are defo before now
				time.Sleep(1 * time.Second)
				require.ErrorIs(err, tt.expectedErr)
			} else {
				require.NoError(err)
				require.Equal(earliest, tt.earliest)
			}
		})
	}
}

func TestUpgradeBytesValidation(t *testing.T) {
	type testRun struct {
		name         string
		upgradesFile []byte
		expectedErr  error
	}

	tests := []testRun{
		{
			name:         "empty file",
			upgradesFile: []byte{},
			expectedErr:  errInvalidPrecompiles,
		},
		{
			name: "empty upgrades",
			upgradesFile: []byte(
				`{"precompileUpgrades":[]}`),
			expectedErr: errNoPrecompiles,
		},
		{
			name: "precompile is not []",
			upgradesFile: []byte(
				`{"precompileUpgrades":{"badPrecompile":1234}}`),
			expectedErr: errInvalidPrecompiles,
		},
		{
			name: "no blockTimestamp",
			upgradesFile: []byte(
				`{"precompileUpgrades":[{"feeManagerConfig":{"initialFeeConfig":{"something":"isset"}}}]}`),
			expectedErr: errNoBlockTimestamp,
		},
		{
			name: "bad blockTimestamp type",
			upgradesFile: []byte(
				`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":"1234","initialFeeConfig":{}}}]}`),
			expectedErr: errInvalidPrecompiles,
		},
		{
			name: "zero blockTimestamp",
			upgradesFile: []byte(
				`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":0,"initialFeeConfig":{}}}]}`),
			expectedErr: errBlockTimestampInvalid,
		},
		{
			name: "blockTimestamp ok",
			upgradesFile: []byte(
				fmt.Sprintf(`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}]}`,
					time.Now().Add(1*time.Minute).Unix()),
			),
			expectedErr: nil,
		},
	}

	skipPrompting := false
	require := require.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateUpgradeBytes(tt.upgradesFile, nil, skipPrompting)
			require.ErrorIs(err, tt.expectedErr)
		})
	}
}

func TestForceIgnorePastTimestamp(t *testing.T) {
	skipPrompting := true
	upgradesFile := []byte(
		`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":1674496268,"initialFeeConfig":{}}}]}`)

	require := require.New(t)
	_, err := validateUpgradeBytes(upgradesFile, nil, skipPrompting)
	require.NoError(err)
}

func TestLockFile(t *testing.T) {
	type testRun struct {
		name         string
		upgradesFile []byte
		lockFile     []byte
		expectedErr  error
	}

	sameActivation := time.Now().Add(1 * time.Minute)

	tests := []testRun{
		{
			name: "same file",
			upgradesFile: []byte(
				fmt.Sprintf(`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}]}`,
					time.Now().Add(1*time.Minute).Unix()),
			),
			lockFile: []byte(
				fmt.Sprintf(`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}]}`,
					time.Now().Add(1*time.Minute).Unix()),
			),
			expectedErr: nil,
		},
		{
			name: "added precompile",
			upgradesFile: []byte(
				fmt.Sprintf(`
{"precompileUpgrades":[
{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"txAllowListConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}},
{"contractNativeMinterConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}
]}`,
					sameActivation.Unix(),
					time.Now().Add(20*time.Second).Unix(),
					time.Now().Add(30*time.Second).Unix(),
				)),
			lockFile: []byte(
				fmt.Sprintf(`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}]}`,
					time.Now().Add(1*time.Minute).Unix()),
			),
			expectedErr: nil,
		},
		{
			name: "empty lock",
			upgradesFile: []byte(
				fmt.Sprintf(`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xcccccccccccccccccccccccccccBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}]}`,
					sameActivation.Unix()),
			),
			lockFile:    []byte{},
			expectedErr: nil,
		},
		{
			name: "altered initial",
			upgradesFile: []byte(
				fmt.Sprintf(`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xcccccccccccccccccccccccccccBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}]}`,
					time.Now().Add(1*time.Minute).Unix()),
			),
			lockFile: []byte(
				fmt.Sprintf(`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":%d,"initialFeeConfig":{}}}]}`,
					time.Now().Add(1*time.Minute).Unix()),
			),
			expectedErr: errNewUpgradesNotContainsLock,
		},
	}

	skipPrompting := false
	require := require.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateUpgradeBytes(tt.upgradesFile, tt.lockFile, skipPrompting)
			require.ErrorIs(err, tt.expectedErr)
		})
	}
}
