// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/mocks"
	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestExportImportSubnet(t *testing.T) {
	testDir := t.TempDir()
	require := require.New(t)
	testSubnet := "testSubnet"
	vmVersion := "v0.9.99"
	testSubnetEVMCompat := []byte("{\"rpcChainVMProtocolVersion\": {\"v0.9.99\": 18}}")

	app = application.New()

	mockAppDownloader := mocks.Downloader{}
	mockAppDownloader.On("Download", mock.Anything).Return(testSubnetEVMCompat, nil)

	app.Setup(testDir, logging.NoLog{}, nil, prompts.NewPrompter(), &mockAppDownloader)
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	genBytes, err := os.ReadFile("../../" + utils.SubnetEvmGenesisPath)
	require.NoError(err)
	sc, err := vm.CreateEvmSidecar(
		app,
		testSubnet,
		vmVersion,
		"Test",
		false,
	)
	require.NoError(err)
	err = app.WriteGenesisFile(testSubnet, genBytes)
	require.NoError(err)
	err = app.CreateSidecar(sc)
	require.NoError(err)

	exportOutputDir := filepath.Join(testDir, "output")
	err = os.MkdirAll(exportOutputDir, constants.DefaultPerms755)
	require.NoError(err)
	exportOutput = filepath.Join(exportOutputDir, testSubnet)
	defer func() {
		exportOutput = ""
		app = nil
	}()
	globalNetworkFlags.UseLocal = true
	err = exportSubnet(nil, []string{"this-does-not-exist-should-fail"})
	require.Error(err)

	err = exportSubnet(nil, []string{testSubnet})
	require.NoError(err)
	require.FileExists(exportOutput)
	sidecarFile := filepath.Join(app.GetBaseDir(), constants.SubnetDir, testSubnet, constants.SidecarFileName)
	orig, err := os.ReadFile(sidecarFile)
	require.NoError(err)

	var control map[string]interface{}
	err = json.Unmarshal(orig, &control)
	require.NoError(err)
	require.Equal(control["Name"], testSubnet)
	require.Equal(control["VM"], "Subnet-EVM")
	require.Equal(control["VMVersion"], vmVersion)
	require.Equal(control["Subnet"], testSubnet)
	require.Equal(control["TokenName"], "Test Token")
	require.Equal(control["TokenSymbol"], "Test")
	require.Equal(control["Version"], constants.SidecarVersion)
	require.Equal(control["Networks"], nil)

	err = os.Remove(sidecarFile)
	require.NoError(err)

	err = importBlockchain(nil, []string{"this-does-also-not-exist-import-should-fail"})
	require.ErrorIs(err, os.ErrNotExist)
	err = importBlockchain(nil, []string{exportOutput})
	require.ErrorContains(err, "subnet already exists")
	genFile := filepath.Join(app.GetBaseDir(), constants.SubnetDir, testSubnet, constants.GenesisFileName)
	err = os.Remove(genFile)
	require.NoError(err)
	err = importBlockchain(nil, []string{exportOutput})
	require.NoError(err)
}
