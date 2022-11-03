package vm

import (
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var testSubnetEVMCompat = []byte("{\"rpcChainVMProtocolVersion\": {\"v0.4.2\": 18,\"v0.4.1\": 18,\"v0.4.0\": 17}}")

func TestGetRPCProtocolVersion(t *testing.T) {
	assert := assert.New(t)
	version := "v0.4.2"
	expectedRPC := 18

	mockDownloader := &mocks.Downloader{}
	mockDownloader.On("Download", mock.Anything).Return(testSubnetEVMCompat, nil)

	app := application.New()
	app.Downloader = mockDownloader

	rpcVersion, err := GetRPCProtocolVersion(app, models.SubnetEvm, version)
	assert.NoError(err)
	assert.Equal(expectedRPC, rpcVersion)
}

func TestGetRPCProtocolVersionMissing(t *testing.T) {
	assert := assert.New(t)
	version := "v0.4.3"

	mockDownloader := &mocks.Downloader{}
	mockDownloader.On("Download", mock.Anything).Return(testSubnetEVMCompat, nil)

	app := application.New()
	app.Downloader = mockDownloader

	_, err := GetRPCProtocolVersion(app, models.SubnetEvm, version)
	assert.ErrorContains(err, "no RPC version found")
}
