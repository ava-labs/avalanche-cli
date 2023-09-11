// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package plugins

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/internal/testutils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/ux"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
)

const (
	subnetName1 = "TEST_subnet"
	subnetName2 = "TEST_copied_subnet"

	subnetID  = "testSubNet"
	networkID = "67443"
)

// testing backward compatibility
func TestEditConfigFileWithOldPattern(t *testing.T) {
	ux.NewUserLog(logging.NoLog{}, io.Discard)

	require := require.New(t)

	ap := testutils.SetupTestInTempDir(t)

	genesisBytes := []byte("genesis")
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	require.NoError(err)

	configFile := constants.NodeFileName

	// Create ConfigFile
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, configFile)
	defer os.Remove(configPath)

	// testing backward compatibility
	configBytes := []byte("{\"whitelisted-subnets\": \"subNetId000\"}")
	err = os.MkdirAll(filepath.Dir(configPath), constants.DefaultPerms755)
	require.NoError(err)
	err = os.WriteFile(configPath, configBytes, 0o600)
	require.NoError(err)

	err = EditConfigFile(ap, subnetID, networkID, configPath, true, "")
	require.NoError(err)

	fileBytes, err := os.ReadFile(configPath)
	require.NoError(err)

	var avagoConfig map[string]interface{}
	err = json.Unmarshal(fileBytes, &avagoConfig)
	require.NoError(err)

	require.Equal("subNetId000,testSubNet", avagoConfig["track-subnets"])

	// ensure that the old setting has been deleted
	require.Equal(nil, avagoConfig["whitelisted-subnets"])
}

// testing backward compatibility
func TestEditConfigFileWithNewPattern(t *testing.T) {
	ux.NewUserLog(logging.NoLog{}, io.Discard)

	require := require.New(t)

	ap := testutils.SetupTestInTempDir(t)

	genesisBytes := []byte("genesis")
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	require.NoError(err)

	configFile := constants.NodeFileName

	// Create ConfigFile
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, configFile)
	defer os.Remove(configPath)

	// testing backward compatibility
	configBytes := []byte("{\"track-subnets\": \"subNetId000\"}")
	err = os.MkdirAll(filepath.Dir(configPath), constants.DefaultPerms755)
	require.NoError(err)
	err = os.WriteFile(configPath, configBytes, 0o600)
	require.NoError(err)

	err = EditConfigFile(ap, subnetID, networkID, configPath, true, "")
	require.NoError(err)

	fileBytes, err := os.ReadFile(configPath)
	require.NoError(err)

	var avagoConfig map[string]interface{}
	err = json.Unmarshal(fileBytes, &avagoConfig)
	require.NoError(err)

	require.Equal("subNetId000,testSubNet", avagoConfig["track-subnets"])

	// ensure that the old setting wont be applied at all
	require.Equal(nil, avagoConfig["whitelisted-subnets"])
}

func TestEditConfigFileWithNoSettings(t *testing.T) {
	ux.NewUserLog(logging.NoLog{}, io.Discard)

	require := require.New(t)

	ap := testutils.SetupTestInTempDir(t)

	genesisBytes := []byte("genesis")
	err := ap.WriteGenesisFile(subnetName1, genesisBytes)
	require.NoError(err)

	configFile := constants.NodeFileName

	// Create ConfigFile
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, configFile)
	defer os.Remove(configPath)

	// testing when no setting for tracked subnets exists
	configBytes := []byte("{\"networkId\": \"5\"}")
	err = os.MkdirAll(filepath.Dir(configPath), constants.DefaultPerms755)
	require.NoError(err)
	err = os.WriteFile(configPath, configBytes, 0o600)
	require.NoError(err)

	err = EditConfigFile(ap, subnetID, networkID, configPath, true, "")
	require.NoError(err)

	fileBytes, err := os.ReadFile(configPath)
	require.NoError(err)

	var avagoConfig map[string]interface{}
	err = json.Unmarshal(fileBytes, &avagoConfig)
	require.NoError(err)

	require.Equal("testSubNet", avagoConfig["track-subnets"])

	// ensure that the old setting wont be applied at all
	require.Equal(nil, avagoConfig["whitelisted-subnets"])
}
