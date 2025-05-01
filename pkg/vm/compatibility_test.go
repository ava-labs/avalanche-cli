// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testAvagoVersion         = "v0.4.2"
	testUnlistedAvagoVersion = "v0.4.3"
)

var testSubnetEVMCompat = []byte("{\"rpcChainVMProtocolVersion\": {\"v0.4.2\": 18,\"v0.4.1\": 18,\"v0.4.0\": 17}}")

func TestGetRPCProtocolVersionSubnetEVM(t *testing.T) {
	require := require.New(t)
	expectedRPC := 18
	var vm models.VMType = models.SubnetEvm

	mockDownloader := &mocks.Downloader{}
	mockDownloader.On("Download", mock.Anything).Return(testSubnetEVMCompat, nil)

	app := application.New()
	app.Downloader = mockDownloader

	rpcVersion, err := GetRPCProtocolVersion(app, vm, testAvagoVersion)
	require.NoError(err)
	require.Equal(expectedRPC, rpcVersion)
}

func TestGetRPCProtocolVersionUnknownVM(t *testing.T) {
	require := require.New(t)
	var vm models.VMType = "unknown"

	app := application.New()

	_, err := GetRPCProtocolVersion(app, vm, testAvagoVersion)
	require.ErrorContains(err, "unknown VM type")
}

func TestGetRPCProtocolVersionMissing(t *testing.T) {
	require := require.New(t)

	mockDownloader := &mocks.Downloader{}
	mockDownloader.On("Download", mock.Anything).Return(testSubnetEVMCompat, nil)

	app := application.New()
	app.Downloader = mockDownloader

	_, err := GetRPCProtocolVersion(app, models.SubnetEvm, testUnlistedAvagoVersion)
	require.ErrorContains(err, "no RPC version found")
}
