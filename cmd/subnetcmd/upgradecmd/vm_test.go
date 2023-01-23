// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package upgradecmd

import (
	"os"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

func TestAtMostOneNetworkSelected(t *testing.T) {
	assert := require.New(t)

	type test struct {
		name       string
		useConfig  bool
		useLocal   bool
		useFuji    bool
		useMainnet bool
		valid      bool
	}

	tests := []test{
		{
			name:       "all false",
			useConfig:  false,
			useLocal:   false,
			useFuji:    false,
			useMainnet: false,
			valid:      true,
		},
		{
			name:       "future true",
			useConfig:  true,
			useLocal:   false,
			useFuji:    false,
			useMainnet: false,
			valid:      true,
		},
		{
			name:       "local true",
			useConfig:  false,
			useLocal:   true,
			useFuji:    false,
			useMainnet: false,
			valid:      true,
		},
		{
			name:       "fuji true",
			useConfig:  false,
			useLocal:   false,
			useFuji:    true,
			useMainnet: false,
			valid:      true,
		},
		{
			name:       "mainnet true",
			useConfig:  false,
			useLocal:   false,
			useFuji:    false,
			useMainnet: true,
			valid:      true,
		},
		{
			name:       "double true 1",
			useConfig:  true,
			useLocal:   true,
			useFuji:    false,
			useMainnet: false,
			valid:      false,
		},
		{
			name:       "double true 2",
			useConfig:  true,
			useLocal:   false,
			useFuji:    true,
			useMainnet: false,
			valid:      false,
		},
		{
			name:       "double true 3",
			useConfig:  true,
			useLocal:   false,
			useFuji:    false,
			useMainnet: true,
			valid:      false,
		},
		{
			name:       "double true 4",
			useConfig:  false,
			useLocal:   true,
			useFuji:    true,
			useMainnet: false,
			valid:      false,
		},
		{
			name:       "double true 5",
			useConfig:  false,
			useLocal:   true,
			useFuji:    false,
			useMainnet: true,
			valid:      false,
		},
		{
			name:       "double true 6",
			useConfig:  false,
			useLocal:   false,
			useFuji:    true,
			useMainnet: true,
			valid:      false,
		},
		{
			name:       "all true",
			useConfig:  true,
			useLocal:   true,
			useFuji:    true,
			useMainnet: true,
			valid:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useConfig = tt.useConfig
			useLocal = tt.useLocal
			useFuji = tt.useFuji
			useMainnet = tt.useMainnet

			accepted := atMostOneNetworkSelected()
			if tt.valid {
				assert.True(accepted)
			} else {
				assert.False(accepted)
			}
		})
	}
}

func TestAtMostOneVersionSelected(t *testing.T) {
	assert := require.New(t)

	type test struct {
		name      string
		useLatest bool
		version   string
		binary    string
		valid     bool
	}

	tests := []test{
		{
			name:      "all empty",
			useLatest: false,
			version:   "",
			binary:    "",
			valid:     true,
		},
		{
			name:      "one selected 1",
			useLatest: true,
			version:   "",
			binary:    "",
			valid:     true,
		},
		{
			name:      "one selected 2",
			useLatest: false,
			version:   "v1.2.0",
			binary:    "",
			valid:     true,
		},
		{
			name:      "one selected 3",
			useLatest: false,
			version:   "",
			binary:    "home",
			valid:     true,
		},
		{
			name:      "two selected 1",
			useLatest: true,
			version:   "v1.2.0",
			binary:    "",
			valid:     false,
		},
		{
			name:      "two selected 2",
			useLatest: true,
			version:   "",
			binary:    "home",
			valid:     false,
		},
		{
			name:      "two selected 3",
			useLatest: false,
			version:   "v1.2.0",
			binary:    "home",
			valid:     false,
		},
		{
			name:      "all selected",
			useLatest: true,
			version:   "v1.2.0",
			binary:    "home",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useLatest = tt.useLatest
			targetVersion = tt.version
			newBinary = tt.binary

			accepted := atMostOneVersionSelected()
			if tt.valid {
				assert.True(accepted)
			} else {
				assert.False(accepted)
			}
		})
	}
}

func TestAtMostOneAutomationSelected(t *testing.T) {
	assert := require.New(t)

	type test struct {
		name      string
		useManual bool
		pluginDir string
		valid     bool
	}

	tests := []test{
		{
			name:      "all empty",
			useManual: false,
			pluginDir: "",
			valid:     true,
		},
		{
			name:      "manual selected",
			useManual: true,
			pluginDir: "",
			valid:     true,
		},
		{
			name:      "auto selected",
			useManual: false,
			pluginDir: "home",
			valid:     true,
		},
		{
			name:      "both selected",
			useManual: true,
			pluginDir: "home",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useManual = tt.useManual
			pluginDir = tt.pluginDir

			accepted := atMostOneAutomationSelected()
			if tt.valid {
				assert.True(accepted)
			} else {
				assert.False(accepted)
			}
		})
	}
}

func TestUpdateToCustomBin(t *testing.T) {
	assert := require.New(t)
	testDir := t.TempDir()

	subnetName := "testSubnet"
	sc := models.Sidecar{
		Name:       subnetName,
		VM:         models.SubnetEvm,
		VMVersion:  "v3.0.0",
		RPCVersion: 20,
		Subnet:     subnetName,
	}
	networkToUpgrade := futureDeployment

	factory := logging.NewFactory(logging.Config{})
	log, err := factory.Make("avalanche")
	assert.NoError(err)

	// create the user facing logger as a global var
	ux.NewUserLog(log, os.Stdout)

	app = &application.Avalanche{}
	app.Setup(testDir, log, config.New(), prompts.NewPrompter(), application.NewDownloader())

	err = os.MkdirAll(app.GetSubnetDir(), constants.DefaultPerms755)
	assert.NoError(err)

	err = app.CreateSidecar(&sc)
	assert.NoError(err)

	err = os.MkdirAll(app.GetCustomVMDir(), constants.DefaultPerms755)
	assert.NoError(err)

	newBinary = "../../../tests/assets/dummyVmBinary.bin"

	assert.FileExists(newBinary)

	err = updateToCustomBin(subnetName, sc, networkToUpgrade)
	assert.NoError(err)

	// check new binary exists and matches
	placedBinaryPath := app.GetCustomVMPath(subnetName)
	assert.FileExists(placedBinaryPath)
	expectedHash, err := utils.GetSHA256FromDisk(newBinary)
	assert.NoError(err)

	actualHash, err := utils.GetSHA256FromDisk(placedBinaryPath)
	assert.NoError(err)

	assert.Equal(expectedHash, actualHash)

	// check sidecar
	diskSC, err := app.LoadSidecar(subnetName)
	assert.NoError(err)
	assert.Equal(models.VMTypeFromString(models.CustomVM), diskSC.VM)
	assert.Empty(diskSC.VMVersion)
}
