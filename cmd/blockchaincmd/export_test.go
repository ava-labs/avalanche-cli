// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package blockchaincmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-cli/pkg/vm"
	"github.com/ava-labs/avalanche-cli/tests/e2e/utils"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/logging"
)

func TestExportImportSubnet(t *testing.T) {
	testDir := t.TempDir()
	require := require.New(t)
	testSubnet := "testSubnet"
	vmVersion := "v0.9.99"

	app = application.New()

	app.Setup(testDir, logging.NoLog{}, nil, prompts.NewPrompter())
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	genBytes, err := vm.LoadCustomGenesis(
		app,
		"../../"+utils.SubnetEvmGenesisPath,
	)
	require.NoError(err)
	sc, err := vm.CreateEVMSidecar(
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
	require.Equal(testSubnet, control["Name"])
	require.Equal("Subnet-EVM", control["VM"])
	require.Equal(vmVersion, control["VMVersion"])
	require.Equal(testSubnet, control["Subnet"])
	require.Equal("Test Token", control["TokenName"])
	require.Equal("Test", control["TokenSymbol"])
	require.Equal(constants.SidecarVersion, control["Version"])
	require.Nil(control["Networks"])

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
