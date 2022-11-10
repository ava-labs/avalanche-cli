// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testAvagoVersion         = "v0.4.2"
	testUnlistedAvagoVersion = "v0.4.3"
)

var (
	testSubnetEVMCompat = []byte("{\"rpcChainVMProtocolVersion\": {\"v0.4.2\": 18,\"v0.4.1\": 18,\"v0.4.0\": 17}}")
	testAvagoCompat     = []byte("{\"19\": [\"v1.9.2\"],\"18\": [\"v1.9.1\"],\"17\": [\"v1.9.0\",\"v1.8.0\"]}")
	testAvagoCompat2    = []byte("{\"19\": [\"v1.9.2\", \"v1.9.1\"],\"18\": [\"v1.9.0\"]}")
	testAvagoCompat3    = []byte("{\"19\": [\"v1.9.2\", \"v1.9.1\"],\"18\": [\"v1.9.0\"]}")
)

func TestGetRPCProtocolVersionSubnetEVM(t *testing.T) {
	assert := assert.New(t)
	expectedRPC := 18
	var vm models.VMType = models.SubnetEvm

	mockDownloader := &mocks.Downloader{}
	mockDownloader.On("Download", mock.Anything).Return(testSubnetEVMCompat, nil)

	app := application.New()
	app.Downloader = mockDownloader

	rpcVersion, err := GetRPCProtocolVersion(app, vm, testAvagoVersion)
	assert.NoError(err)
	assert.Equal(expectedRPC, rpcVersion)
}

func TestGetRPCProtocolVersionSpacesVM(t *testing.T) {
	assert := assert.New(t)
	expectedRPC := 18
	var vm models.VMType = models.SpacesVM

	mockDownloader := &mocks.Downloader{}
	mockDownloader.On("Download", mock.Anything).Return(testSubnetEVMCompat, nil)

	app := application.New()
	app.Downloader = mockDownloader

	rpcVersion, err := GetRPCProtocolVersion(app, vm, testAvagoVersion)
	assert.NoError(err)
	assert.Equal(expectedRPC, rpcVersion)
}

func TestGetRPCProtocolVersionUnknownVM(t *testing.T) {
	assert := assert.New(t)
	var vm models.VMType = "unknown"

	app := application.New()

	_, err := GetRPCProtocolVersion(app, vm, testAvagoVersion)
	assert.ErrorContains(err, "unknown VM type")
}

func TestGetRPCProtocolVersionMissing(t *testing.T) {
	assert := assert.New(t)

	mockDownloader := &mocks.Downloader{}
	mockDownloader.On("Download", mock.Anything).Return(testSubnetEVMCompat, nil)

	app := application.New()
	app.Downloader = mockDownloader

	_, err := GetRPCProtocolVersion(app, models.SubnetEvm, testUnlistedAvagoVersion)
	assert.ErrorContains(err, "no RPC version found")
}

func TestGetLatestAvalancheGoByProtocolVersion(t *testing.T) {
	type versionTest struct {
		rpc             int
		testData        []byte
		latestVersion   string
		expectedVersion string
		expectedErr     error
	}

	tests := []versionTest{
		{
			rpc:             19,
			testData:        testAvagoCompat,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.2",
			expectedErr:     nil,
		},
		{
			rpc:             18,
			testData:        testAvagoCompat,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.1",
			expectedErr:     nil,
		},
		{
			rpc:             19,
			testData:        testAvagoCompat2,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.2",
			expectedErr:     nil,
		},
		{
			rpc:             19,
			testData:        testAvagoCompat3,
			latestVersion:   "v1.9.2",
			expectedVersion: "v1.9.2",
			expectedErr:     nil,
		},
		{
			rpc:             19,
			testData:        testAvagoCompat2,
			latestVersion:   "v1.9.1",
			expectedVersion: "v1.9.1",
			expectedErr:     nil,
		},
		{
			rpc:             20,
			testData:        testAvagoCompat2,
			latestVersion:   "v1.9.2",
			expectedVersion: "",
			expectedErr:     ErrNoAvagoVersion,
		},
		{
			rpc:             19,
			testData:        testAvagoCompat,
			latestVersion:   "v1.9.1",
			expectedVersion: "",
			expectedErr:     ErrNoAvagoVersion,
		},
	}
	for _, tt := range tests {
		assert := assert.New(t)

		mockDownloader := &mocks.Downloader{}
		mockDownloader.On("Download", mock.Anything).Return(tt.testData, nil)
		mockDownloader.On("GetLatestReleaseVersion", mock.Anything).Return(tt.expectedVersion, nil)

		app := application.New()
		app.Downloader = mockDownloader

		avagoVersion, err := GetLatestAvalancheGoByProtocolVersion(app, tt.rpc)
		if tt.expectedErr == nil {
			assert.NoError(err)
		} else {
			assert.ErrorIs(err, tt.expectedErr)
		}
		assert.Equal(tt.expectedVersion, avagoVersion)
	}
}
