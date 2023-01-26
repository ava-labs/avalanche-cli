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
					time.Now().Add(50*time.Millisecond).Unix(),
					time.Now().Add(60*time.Millisecond).Unix(),
					time.Now().Add(70*time.Millisecond).Unix(),
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
			earliest, err := getEarliestTimestamp(upgrades)
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
			name: "blockTimestamp in the past",
			upgradesFile: []byte(
				`{"precompileUpgrades":[{"feeManagerConfig":{"adminAddresses":["0xb794F5eA0ba39494cE839613fffBA74279579268"],"blockTimestamp":1674496268,"initialFeeConfig":{}}}]}`),
			expectedErr: errBlockTimestampInthePast,
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

	require := require.New(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUpgradeBytes(tt.upgradesFile)
			require.ErrorIs(err, tt.expectedErr)
		})
	}
}
