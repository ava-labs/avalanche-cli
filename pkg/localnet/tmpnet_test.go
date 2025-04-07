// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package localnet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ava-labs/avalanche-cli/pkg/application"
	"github.com/ava-labs/avalanche-cli/pkg/config"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/prompts"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/stretchr/testify/require"
)

func createFile(t *testing.T, path string) {
	f, err := os.Create(path)
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)
}

func TestGetTmpNetAvailableLogs(t *testing.T) {
	app := &application.Avalanche{}
	appDir, err := os.MkdirTemp(os.TempDir(), "cli-app-test")
	require.NoError(t, err)
	app.Setup(appDir, logging.NoLog{}, config.New(), "", prompts.NewPrompter(), application.NewDownloader())
	networkID, unparsedGenesis, upgradeBytes, defaultFlags, nodes, err := GetDefaultNetworkConf(2)
	require.NoError(t, err)
	networkDir, err := os.MkdirTemp(os.TempDir(), "cli-tmpnet-test")
	require.NoError(t, err)
	_, err = TmpNetCreate(
		context.Background(),
		app.Log,
		networkDir,
		"",
		"",
		networkID,
		nil,
		nil,
		unparsedGenesis,
		upgradeBytes,
		defaultFlags,
		nodes,
		false,
	)
	require.NoError(t, err)
	// no logs yet
	logPaths, err := GetTmpNetAvailableLogs(networkDir, ids.Empty, false)
	require.NoError(t, err)
	require.Equal(t, []string{}, logPaths)
	// default network
	node1ID := "NodeID-7Xhw2mDxuDS44j42TCB6U5579esbSt3Lg"
	node2ID := "NodeID-MFrZFVCXPv5iCn6M9K6XduxGTYp891xXZ"
	node1Logs := filepath.Join(networkDir, node1ID, "logs")
	node2Logs := filepath.Join(networkDir, node2ID, "logs")
	err = os.MkdirAll(node1Logs, constants.DefaultPerms755)
	require.NoError(t, err)
	err = os.MkdirAll(node2Logs, constants.DefaultPerms755)
	require.NoError(t, err)
	// add main log
	createFile(t, filepath.Join(node1Logs, "main.log"))
	createFile(t, filepath.Join(node2Logs, "main.log"))
	logPaths, err = GetTmpNetAvailableLogs(networkDir, ids.Empty, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
	// add P chain log
	createFile(t, filepath.Join(node1Logs, "P.log"))
	createFile(t, filepath.Join(node2Logs, "P.log"))
	logPaths, err = GetTmpNetAvailableLogs(networkDir, ids.Empty, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, "P.log"),
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, "P.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
	// gather C chain when no files are present
	logPaths, err = GetTmpNetAvailableLogs(networkDir, ids.Empty, true)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, "P.log"),
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, "P.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
	// gather C chain when files are present
	createFile(t, filepath.Join(node1Logs, "C.log"))
	createFile(t, filepath.Join(node2Logs, "C.log"))
	logPaths, err = GetTmpNetAvailableLogs(networkDir, ids.Empty, true)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, "C.log"),
		filepath.Join(node1Logs, "P.log"),
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, "C.log"),
		filepath.Join(node2Logs, "P.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
	// don't gather C chain when files are present
	logPaths, err = GetTmpNetAvailableLogs(networkDir, ids.Empty, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, "P.log"),
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, "P.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
	// gather blockchain when no files are present
	blockchainID := ids.GenerateTestID()
	logPaths, err = GetTmpNetAvailableLogs(networkDir, blockchainID, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, "P.log"),
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, "P.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
	// gather blockchain when files are present
	createFile(t, filepath.Join(node1Logs, blockchainID.String()+".log"))
	createFile(t, filepath.Join(node2Logs, blockchainID.String()+".log"))
	logPaths, err = GetTmpNetAvailableLogs(networkDir, blockchainID, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, blockchainID.String()+".log"),
		filepath.Join(node1Logs, "P.log"),
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, blockchainID.String()+".log"),
		filepath.Join(node2Logs, "P.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
	// don't gather blockchain when files are present
	logPaths, err = GetTmpNetAvailableLogs(networkDir, ids.Empty, false)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, "P.log"),
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, "P.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
	// gather all files are present
	logPaths, err = GetTmpNetAvailableLogs(networkDir, blockchainID, true)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(node1Logs, blockchainID.String()+".log"),
		filepath.Join(node1Logs, "C.log"),
		filepath.Join(node1Logs, "P.log"),
		filepath.Join(node1Logs, "main.log"),
		filepath.Join(node2Logs, blockchainID.String()+".log"),
		filepath.Join(node2Logs, "C.log"),
		filepath.Join(node2Logs, "P.log"),
		filepath.Join(node2Logs, "main.log"),
	}, logPaths)
}
