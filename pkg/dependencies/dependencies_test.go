// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dependencies

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testAvagoCompat  = []byte("{\"19\": [\"v1.9.2\"],\"18\": [\"v1.9.1\"],\"17\": [\"v1.9.0\",\"v1.8.0\"]}")
	testAvagoCompat2 = []byte("{\"19\": [\"v1.9.2\", \"v1.9.1\"],\"18\": [\"v1.9.0\"]}")
	testAvagoCompat3 = []byte("{\"19\": [\"v1.9.1\", \"v1.9.2\"],\"18\": [\"v1.9.0\"]}")
	testAvagoCompat4 = []byte("{\"19\": [\"v1.9.1\", \"v1.9.2\", \"v1.9.11\"],\"18\": [\"v1.9.0\"]}")
	testAvagoCompat5 = []byte("{\"39\": [\"v1.12.2\", \"v1.13.0\"],\"38\": [\"v1.11.13\", \"v1.12.0\", \"v1.12.1\"]}")
	testAvagoCompat6 = []byte("{\"39\": [\"v1.12.2\", \"v1.13.0\", \"v1.13.1\"],\"38\": [\"v1.11.13\", \"v1.12.0\", \"v1.12.1\"]}")
	testAvagoCompat7 = []byte("{\"40\": [\"v1.13.2\"],\"39\": [\"v1.12.2\", \"v1.13.0\", \"v1.13.1\"]}")
	testCLICompat    = []byte(`{"subnet-evm":"v0.7.3","rpc":39,"avalanchego":{"Local Network":{"latest-version":"v1.13.0"},"DevNet":{"latest-version":"v1.13.0"},"Fuji":{"latest-version":"v1.13.0"},"Mainnet":{"latest-version":"v1.13.0"}}, "signature-aggregator": "signature-aggregator-v0.4.4"}`)
	testCLICompat2   = []byte(`{"subnet-evm":"v0.7.3","rpc":39,"avalanchego":{"Local Network":{"latest-version":"v1.13.0"},"DevNet":{"latest-version":"v1.13.0"},"Fuji":{"latest-version":"v1.13.0-fuji"},"Mainnet":{"latest-version":"v1.13.0"}}, "signature-aggregator": "signature-aggregator-v0.4.4"}`)
)

func TestGetLatestAvalancheGoByProtocolVersion(t *testing.T) {
	type versionTest struct {
		name            string
		rpc             int
		testData        []byte
		latestVersion   string
		expectedVersion string
		expectedErr     error
	}

	tests := []versionTest{
		{
			name:            "latest, one entry",
			rpc:             19,
			testData:        testAvagoCompat,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.2",
			expectedErr:     nil,
		},
		{
			name:            "older, one entry",
			rpc:             18,
			testData:        testAvagoCompat,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.1",
			expectedErr:     nil,
		},
		{
			name:            "latest, multiple entry",
			rpc:             19,
			testData:        testAvagoCompat2,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.2",
			expectedErr:     nil,
		},
		{
			name:            "latest, multiple entry, reverse sorted",
			rpc:             19,
			testData:        testAvagoCompat3,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.2",
			expectedErr:     nil,
		},
		{
			name:            "latest, multiple entry, unreleased version",
			rpc:             19,
			testData:        testAvagoCompat2,
			latestVersion:   "v1.9.1",
			expectedVersion: "v1.9.1",
			expectedErr:     nil,
		},
		{
			name:            "no rpc version",
			rpc:             20,
			testData:        testAvagoCompat2,
			latestVersion:   "v1.9.2",
			expectedVersion: "",
			expectedErr:     ErrNoAvagoVersion,
		},
		{
			name:            "existing rpc, but no eligible version",
			rpc:             19,
			testData:        testAvagoCompat,
			latestVersion:   "v1.9.1",
			expectedVersion: "",
			expectedErr:     ErrNoAvagoVersion,
		},
		{
			name:            "string sorting test",
			rpc:             19,
			testData:        testAvagoCompat4,
			latestVersion:   "v1.9.11",
			expectedVersion: "v1.9.11",
			expectedErr:     nil,
		},
		{
			name:            "string sorting test 2",
			rpc:             19,
			testData:        testAvagoCompat4,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.2",
			expectedErr:     nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			mockDownloader := &mocks.Downloader{}
			mockDownloader.On("Download", mock.Anything).Return(tt.testData, nil)
			mockDownloader.On("GetLatestReleaseVersion", mock.Anything, mock.Anything, mock.Anything).Return(tt.latestVersion, nil)

			app := application.New()
			app.Downloader = mockDownloader

			avagoVersion, err := GetLatestAvalancheGoByProtocolVersion(app, tt.rpc)
			if tt.expectedErr == nil {
				require.NoError(err)
			} else {
				require.ErrorIs(err, tt.expectedErr)
			}
			require.Equal(tt.expectedVersion, avagoVersion)
		})
	}
}

func TestGetLatestCLISupportedDependencyVersion(t *testing.T) {
	tests := []struct {
		name              string
		dependency        string
		expectedError     bool
		expectedResult    string
		cliDependencyData []byte
		avalancheGoData   []byte
		latestVersion     string
	}{
		{
			name:              "avalanchego dependency with cli supporting latest avalanchego release",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat,
			avalancheGoData:   testAvagoCompat5,
			latestVersion:     "v1.13.0",
			expectedError:     false,
			expectedResult:    "v1.13.0",
		},
		{
			name:              "avalanchego dependency with cli not supporting latest avalanchego release, but same rpc",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat,
			avalancheGoData:   testAvagoCompat6,
			latestVersion:     "v1.13.1",
			expectedError:     false,
			expectedResult:    "v1.13.0",
		},
		{
			name:              "avalanchego dependency with cli supporting lower rpc",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat,
			avalancheGoData:   testAvagoCompat7,
			latestVersion:     "v1.13.2",
			expectedError:     false,
			expectedResult:    "v1.13.0",
		},
		{
			name:              "avalanchego dependency with cli requiring a prerelease",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat2,
			avalancheGoData:   testAvagoCompat7,
			latestVersion:     "v1.13.2",
			expectedError:     false,
			expectedResult:    "v1.13.0-fuji",
		},
		{
			name:              "subnet-evm dependency, where cli latest.json doesn't support newest subnet evm version yet",
			dependency:        constants.SubnetEVMRepoName,
			cliDependencyData: testCLICompat,
			expectedError:     false,
			expectedResult:    "v0.7.3",
			latestVersion:     "v0.7.4",
		},
		{
			name:              "subnet-evm dependency, where cli supports newest subnet evm version",
			dependency:        constants.SubnetEVMRepoName,
			cliDependencyData: testCLICompat,
			expectedError:     false,
			expectedResult:    "v0.7.3",
			latestVersion:     "v0.7.3",
		},
		{
			name:              "signature-aggregator dependency, where cli latest.json doesn't support newest signature-aggregator version yet",
			dependency:        constants.SignatureAggregatorRepoName,
			cliDependencyData: testCLICompat,
			expectedError:     false,
			expectedResult:    "signature-aggregator-v0.4.4",
			latestVersion:     "signature-aggregator-v0.4.5",
		},
		{
			name:              "signature-aggregator dependency, where cli supports newest signature-aggregator version",
			dependency:        constants.SignatureAggregatorRepoName,
			cliDependencyData: testCLICompat,
			expectedError:     false,
			expectedResult:    "signature-aggregator-v0.4.4",
			latestVersion:     "signature-aggregator-v0.4.4",
		},
		{
			name:           "empty dependency",
			dependency:     "",
			expectedError:  true,
			expectedResult: "",
		},
		{
			name:           "invalid dependency",
			dependency:     "invalid",
			expectedError:  true,
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		mockDownloader := &mocks.Downloader{}
		mockDownloader.On("Download", mock.MatchedBy(func(url string) bool {
			return url == constants.CLILatestDependencyURL
		})).Return(tt.cliDependencyData, nil)

		mockDownloader.On("Download", mock.MatchedBy(func(url string) bool {
			return url == constants.AvalancheGoCompatibilityURL
		})).Return(tt.avalancheGoData, nil)
		mockDownloader.On("GetLatestReleaseVersion", mock.Anything, mock.Anything, mock.Anything).Return(tt.latestVersion, nil)

		app := application.New()
		app.Downloader = mockDownloader

		t.Run(tt.name, func(t *testing.T) {
			rpcVersion := 39
			result, err := GetLatestCLISupportedDependencyVersion(app, tt.dependency, models.NewFujiNetwork(), &rpcVersion)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestGetLatestCLISupportedDependencyVersionWithLowerRPC(t *testing.T) {
	tests := []struct {
		name              string
		dependency        string
		expectedError     bool
		expectedResult    string
		cliDependencyData []byte
		avalancheGoData   []byte
		latestVersion     string
	}{
		{
			name:              "avalanchego dependency with cli supporting latest avalanchego release, user using lower rpc",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat,
			avalancheGoData:   testAvagoCompat5,
			expectedError:     false,
			expectedResult:    "v1.12.1",
			latestVersion:     "v1.13.0",
		},
		{
			name:              "avalanchego dependency with cli supporting latest avalanchego release, user using lower rpc, prerelease required",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat2,
			avalancheGoData:   testAvagoCompat6,
			expectedError:     false,
			expectedResult:    "v1.12.1",
			latestVersion:     "v1.13.2",
		},
		{
			name:              "subnet-evm dependency, where cli supports newest subnet evm version",
			dependency:        constants.SubnetEVMRepoName,
			cliDependencyData: testCLICompat,
			expectedError:     false,
			expectedResult:    "v0.7.3",
			latestVersion:     "v0.7.3",
		},
		{
			name:           "empty dependency",
			dependency:     "",
			expectedError:  true,
			expectedResult: "",
		},
		{
			name:           "invalid dependency",
			dependency:     "invalid",
			expectedError:  true,
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		mockDownloader := &mocks.Downloader{}
		mockDownloader.On("Download", mock.MatchedBy(func(url string) bool {
			return url == constants.CLILatestDependencyURL
		})).Return(tt.cliDependencyData, nil)

		mockDownloader.On("Download", mock.MatchedBy(func(url string) bool {
			return url == constants.AvalancheGoCompatibilityURL
		})).Return(tt.avalancheGoData, nil)
		mockDownloader.On("GetLatestReleaseVersion", mock.Anything, mock.Anything, mock.Anything).Return(tt.latestVersion, nil)

		app := application.New()
		app.Downloader = mockDownloader

		t.Run(tt.name, func(t *testing.T) {
			rpcVersion := 38
			result, err := GetLatestCLISupportedDependencyVersion(app, tt.dependency, models.NewFujiNetwork(), &rpcVersion)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
