// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package subnetcmd

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
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/assert"
)

func TestExportImportSubnet(t *testing.T) {
	testDir := t.TempDir()
	assert := assert.New(t)
	testSubnet := "testSubnet"
	vmVersion := "v0.9.99"

	app = application.New()
	app.Setup(testDir, logging.NoLog{}, nil, prompts.NewPrompter())
	ux.NewUserLog(logging.NoLog{}, io.Discard)
	genBytes, sc, err := vm.CreateEvmSubnetConfig(app, testSubnet, "../../"+utils.SubnetEvmGenesisPath, vmVersion)
	assert.NoError(err)
	err = app.WriteGenesisFile(testSubnet, genBytes)
	assert.NoError(err)
	err = app.CreateSidecar(sc)
	assert.NoError(err)

	exportOutputDir := filepath.Join(testDir, "output")
	err = os.MkdirAll(exportOutputDir, constants.DefaultPerms755)
	assert.NoError(err)
	exportOutput = filepath.Join(exportOutputDir, testSubnet)
	defer func() {
		exportOutput = ""
		app = nil
	}()

	err = exportSubnet(nil, []string{"this-does-not-exist-should-fail"})
	assert.Error(err)

	err = exportSubnet(nil, []string{testSubnet})
	assert.NoError(err)
	assert.FileExists(exportOutput)
	sidecarFile := filepath.Join(app.GetBaseDir(), constants.SubnetDir, testSubnet, constants.SidecarFileName)
	orig, err := os.ReadFile(sidecarFile)
	assert.NoError(err)

	var control map[string]interface{}
	err = json.Unmarshal(orig, &control)
	assert.NoError(err)
	assert.Equal(control["Name"], testSubnet)
	assert.Equal(control["VM"], "SubnetEVM")
	assert.Equal(control["VMVersion"], vmVersion)
	assert.Equal(control["Subnet"], testSubnet)
	assert.Equal(control["TokenName"], "TEST")
	assert.Equal(control["Version"], constants.SidecarVersion)
	assert.Equal(control["Networks"], nil)

	err = os.Remove(sidecarFile)
	assert.NoError(err)

	err = importSubnet(nil, []string{"this-does-also-not-exist-import-should-fail"})
	assert.ErrorIs(err, os.ErrNotExist)
	err = importSubnet(nil, []string{exportOutput})
	assert.ErrorContains(err, "subnet already exists")
	genFile := filepath.Join(app.GetBaseDir(), constants.SubnetDir, testSubnet, constants.GenesisFileName)
	err = os.Remove(genFile)
	assert.NoError(err)
	err = importSubnet(nil, []string{exportOutput})
	assert.NoError(err)
}
