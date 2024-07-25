// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"errors"
	"testing"

	"github.com/ava-labs/avalanche-cli/cmd/flags"
	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testAvagoVersion1      = "v1.9.2"
	testAvagoVersion2      = "v1.9.1"
	testLatestAvagoVersion = "latest"
)

var testAvagoCompat = []byte("{\"19\": [\"v1.9.2\"],\"18\": [\"v1.9.1\"],\"17\": [\"v1.9.0\",\"v1.8.0\"]}")

func TestMutuallyExclusive(t *testing.T) {
	require := require.New(t)
	type test struct {
		flagA       bool
		flagB       bool
		flagC       bool
		expectError bool
	}

	tests := []test{
		{
			flagA:       false,
			flagB:       false,
			flagC:       false,
			expectError: false,
		},
		{
			flagA:       true,
			flagB:       false,
			flagC:       false,
			expectError: false,
		},
		{
			flagA:       false,
			flagB:       true,
			flagC:       false,
			expectError: false,
		},
		{
			flagA:       false,
			flagB:       false,
			flagC:       true,
			expectError: false,
		},
		{
			flagA:       true,
			flagB:       false,
			flagC:       true,
			expectError: true,
		},
		{
			flagA:       false,
			flagB:       true,
			flagC:       true,
			expectError: true,
		},
		{
			flagA:       true,
			flagB:       true,
			flagC:       false,
			expectError: true,
		},
		{
			flagA:       true,
			flagB:       true,
			flagC:       true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		isEx := flags.EnsureMutuallyExclusive([]bool{tt.flagA, tt.flagB, tt.flagC})
		if tt.expectError {
			require.False(isEx)
		} else {
			require.True(isEx)
		}
	}
}

func TestCheckForInvalidDeployAndSetAvagoVersion(t *testing.T) {
	type test struct {
		name            string
		networkRPC      int
		networkVersion  string
		networkErr      error
		networkUp       bool
		desiredRPC      int
		desiredVersion  string
		compatData      []byte
		expectError     bool
		expectedVersion string
		compatError     error
	}

	tests := []test{
		{
			name:            "network already running, rpc matches",
			networkRPC:      18,
			networkVersion:  testAvagoVersion1,
			networkErr:      nil,
			desiredRPC:      18,
			desiredVersion:  testLatestAvagoVersion,
			expectError:     false,
			expectedVersion: testAvagoVersion1,
			networkUp:       true,
		},
		{
			name:            "network already running, rpc mismatch",
			networkRPC:      18,
			networkVersion:  testAvagoVersion1,
			networkErr:      nil,
			desiredRPC:      19,
			desiredVersion:  testLatestAvagoVersion,
			expectError:     true,
			expectedVersion: "",
			networkUp:       true,
		},
		{
			name:            "network already running, version mismatch",
			networkRPC:      18,
			networkVersion:  testAvagoVersion1,
			networkErr:      nil,
			desiredRPC:      19,
			desiredVersion:  testAvagoVersion2,
			expectError:     true,
			expectedVersion: "",
			networkUp:       true,
		},
		{
			name:            "network stopped, no err",
			networkRPC:      0,
			networkVersion:  "",
			networkErr:      nil,
			desiredRPC:      19,
			desiredVersion:  testLatestAvagoVersion,
			expectError:     false,
			expectedVersion: testAvagoVersion1,
			compatData:      testAvagoCompat,
			compatError:     nil,
			networkUp:       false,
		},
		{
			name:            "network stopped, no compat",
			networkRPC:      0,
			networkVersion:  "",
			networkErr:      nil,
			desiredRPC:      19,
			desiredVersion:  testLatestAvagoVersion,
			expectError:     false,
			expectedVersion: testAvagoVersion1,
			compatData:      nil,
			compatError:     errors.New("no compat"),
			networkUp:       false,
		},
		{
			name:            "network up, network err",
			networkRPC:      0,
			networkVersion:  "",
			networkErr:      errors.New("unable to determine rpc version"),
			desiredRPC:      19,
			desiredVersion:  testLatestAvagoVersion,
			expectError:     true,
			expectedVersion: testAvagoVersion1,
			compatData:      testAvagoCompat,
			compatError:     nil,
			networkUp:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			mockSC := mocks.StatusChecker{}
			mockSC.On("GetCurrentNetworkVersion").Return(tt.networkVersion, tt.networkRPC, tt.networkUp, tt.networkErr)

			userProvidedAvagoVersion = tt.desiredVersion

			mockDownloader := &mocks.Downloader{}
			mockDownloader.On("Download", mock.Anything).Return(tt.compatData, nil)
			mockDownloader.On("GetLatestReleaseVersion", mock.Anything).Return(tt.expectedVersion, nil)
			mockDownloader.On("GetLatestPreReleaseVersion", mock.Anything, mock.Anything).Return(tt.expectedVersion, nil)

			app = application.New()
			app.Log = logging.NoLog{}
			app.Downloader = mockDownloader

			desiredAvagoVersion, err := CheckForInvalidDeployAndGetAvagoVersion(&mockSC, tt.desiredRPC)

			if tt.expectError {
				require.Error(err)
			} else {
				require.NoError(err)
				require.Equal(tt.expectedVersion, desiredAvagoVersion)
			}
		})
	}
}
