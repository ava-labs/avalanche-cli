// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package version

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCheckMinDependencyVersion(t *testing.T) {
	tests := []struct {
		name              string
		cliVersion        string
		expectedError     bool
		cliDependencyData []byte
	}{
		{
			name:              "cli version equal to minimum version",
			cliVersion:        "v1.8.10",
			cliDependencyData: []byte(`{"min-version":"v1.8.10"}`),
			expectedError:     false,
		},
		{
			name:              "cli version higher than minimum version",
			cliVersion:        "v1.8.11",
			cliDependencyData: []byte(`{"min-version":"v1.8.10"}`),
			expectedError:     false,
		},
		{
			name:              "cli version lower than minimum version",
			cliVersion:        "v1.8.9",
			cliDependencyData: []byte(`{"min-version":"v1.8.10"}`),
			expectedError:     true,
		},
		{
			name:              "cli version much higher than minimum version",
			cliVersion:        "v1.13.0",
			cliDependencyData: []byte(`{"min-version":"v1.8.10"}`),
			expectedError:     false,
		},
		{
			name:              "cli version much lower than minimum version",
			cliVersion:        "v1.7.0",
			cliDependencyData: []byte(`{"min-version":"v1.8.10"}`),
			expectedError:     true,
		},
	}

	for _, tt := range tests {
		mockDownloader := &mocks.Downloader{}
		mockDownloader.On("Download", mock.MatchedBy(func(url string) bool {
			return url == constants.CLIMinVersionURL
		})).Return(tt.cliDependencyData, nil)

		app := application.New()
		app.Downloader = mockDownloader

		t.Run(tt.name, func(t *testing.T) {
			err := CheckCLIVersionIsOverMin(app, tt.cliVersion)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
