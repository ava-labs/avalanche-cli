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

const (
	testAvagoVersion         = "v0.4.2"
	testUnlistedAvagoVersion = "v0.4.3"
)

var (
	testAvagoCompat  = []byte("{\"19\": [\"v1.9.2\"],\"18\": [\"v1.9.1\"],\"17\": [\"v1.9.0\",\"v1.8.0\"]}")
	testAvagoCompat2 = []byte("{\"19\": [\"v1.9.2\", \"v1.9.1\"],\"18\": [\"v1.9.0\"]}")
	testAvagoCompat3 = []byte("{\"19\": [\"v1.9.1\", \"v1.9.2\"],\"18\": [\"v1.9.0\"]}")
	testAvagoCompat4 = []byte("{\"19\": [\"v1.9.1\", \"v1.9.2\", \"v1.9.11\"],\"18\": [\"v1.9.0\"]}")
	testAvagoCompat5 = []byte("{\"39\": [\"v1.12.2\", \"v1.13.0\"],\"38\": [\"v1.11.13\", \"v1.12.0\", \"v1.12.1\"]}")
	testAvagoCompat6 = []byte("{\"39\": [\"v1.12.2\", \"v1.13.0\", \"v1.13.1\"],\"38\": [\"v1.11.13\", \"v1.12.0\", \"v1.12.1\"]}")
	testAvagoCompat7 = []byte("{\"40\": [\"v1.13.2\"],\"39\": [\"v1.12.2\", \"v1.13.0\", \"v1.13.1\"]}")
	testCLICompat    = []byte(`{"subnet-evm":"v0.7.3","rpc":39,"avalanchego":{"Local Network":{"latest-version":"v1.13.0"},"DevNet":{"latest-version":"v1.13.0"},"Fuji":{"latest-version":"v1.13.0"},"Mainnet":{"latest-version":"v1.13.0"}}}`)
	testCLICompat2   = []byte(`{"subnet-evm":"v0.7.3","rpc":39,"avalanchego":{"Local Network":{"latest-version":"v1.13.0"},"DevNet":{"latest-version":"v1.13.0"},"Fuji":{"latest-version":"v1.13.0-fuji"},"Mainnet":{"latest-version":"v1.13.0"}}}`)
)

func TestCheckMinDependencyVersion(t *testing.T) {
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
			name:              "custom avalanchego dependency equal to cli minimum supported version of avalanchego",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat,
			avalancheGoData:   testAvagoCompat5,
			latestVersion:     "v1.13.0",
			expectedError:     false,
			expectedResult:    "v1.13.0",
		},
		{
			name:              "custom avalanchego dependency equal to cli minimum supported version of avalanchego",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat,
			avalancheGoData:   testAvagoCompat6,
			latestVersion:     "v1.13.1",
			expectedError:     false,
			expectedResult:    "v1.13.0",
		},
		{
			name:              "custom avalanchego dependency higher than cli minimum supported version of avalanchego",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLICompat,
			avalancheGoData:   testAvagoCompat7,
			latestVersion:     "v1.13.2",
			expectedError:     false,
			expectedResult:    "v1.13.0",
		},
		{
			name:              "custom avalanchego dependency lower than cli minimum supported version of avalanchego",
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
