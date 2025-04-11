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
	testCLIMinVersion = []byte(`{"subnet-evm":"v0.7.3","rpc":39,"avalanchego":{"Local Network":{"latest-version":"v1.13.0", "minimum-version":""},"DevNet":{"latest-version":"v1.13.0", "minimum-version":""},"Fuji":{"latest-version":"v1.13.0", "minimum-version":"v1.13.0-fuji"},"Mainnet":{"latest-version":"v1.13.0", "minimum-version":"v1.13.0"}}}`)
)

func TestCheckMinDependencyVersion(t *testing.T) {
	tests := []struct {
		name              string
		dependency        string
		expectedError     bool
		cliDependencyData []byte
		customVersion     string
	}{
		{
			name:              "custom avalanchego dependency equal to cli minimum supported version of avalanchego",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLIMinVersion,
			expectedError:     false,
			customVersion:     "v1.13.0-fuji",
		},
		{
			name:              "custom avalanchego dependency higher than cli minimum supported version of avalanchego",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLIMinVersion,
			expectedError:     false,
			customVersion:     "v1.13.0",
		},
		{
			name:              "custom avalanchego dependency lower than cli minimum supported version of avalanchego",
			dependency:        constants.AvalancheGoRepoName,
			cliDependencyData: testCLIMinVersion,
			expectedError:     true,
			customVersion:     "v1.12.2",
		},
	}

	for _, tt := range tests {
		mockDownloader := &mocks.Downloader{}
		mockDownloader.On("Download", mock.MatchedBy(func(url string) bool {
			return url == constants.CLILatestDependencyURL
		})).Return(tt.cliDependencyData, nil)

		app := application.New()
		app.Downloader = mockDownloader

		t.Run(tt.name, func(t *testing.T) {
			err := CheckVersionIsOverMin(app, tt.dependency, models.NewFujiNetwork(), tt.customVersion)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
