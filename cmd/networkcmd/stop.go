// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/teleporter"
	"github.com/ava-labs/avalanche-cli/pkg/utils"
	"github.com/ava-labs/avalanche-cli/pkg/ux"
	"github.com/ava-labs/avalanche-network-runner/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var dontSave bool

func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running local network and preserve state",
		Long: `The network stop command shuts down your local, multi-node network.

All deployed Subnets shutdown gracefully and save their state. If you provide the
--snapshot-name flag, the network saves its state under this named snapshot. You can
reload this snapshot with network start --snapshot-name <snapshotName>. Otherwise, the
network saves to the default snapshot, overwriting any existing state. You can reload the
default snapshot with network start.`,

		RunE: StopNetwork,
		Args: cobrautils.ExactArgs(0),
	}
	cmd.Flags().StringVar(&snapshotName, "snapshot-name", constants.DefaultSnapshotName, "name of snapshot to use to save network state into")
	cmd.Flags().BoolVar(&dontSave, "dont-save", false, "do not save snapshot, just stop the network")
	return cmd
}

func StopNetwork(*cobra.Command, []string) error {
	if err := stopAndSaveNetwork(dontSave); errors.Is(err, binutils.ErrGRPCTimeout) {
		// no server to kill
		return nil
	}

	var err error
	if err = binutils.KillgRPCServerProcess(app); err != nil {
		app.Log.Warn("failed killing server process", zap.Error(err))
		fmt.Println(err)
	} else {
		ux.Logger.PrintToUser("Server shutdown gracefully")
	}

	if err := teleporter.RelayerCleanup(
		app.GetAWMRelayerRunPath(),
		app.GetAWMRelayerStorageDir(),
	); err != nil {
		return err
	}

	return nil
}

func stopAndSaveNetwork(dontSave bool) error {
	cli, err := binutils.NewGRPCClient(
		binutils.WithAvoidRPCVersionCheck(true),
		binutils.WithDialTimeout(constants.FastGRPCDialTimeout),
	)
	if err != nil {
		return err
	}

	ctx, cancel := utils.GetANRContext()
	defer cancel()

	if dontSave {
		_, err = cli.Stop(ctx)
	} else {
		_, err = cli.SaveSnapshot(ctx, snapshotName, true)
	}
	if err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			ux.Logger.PrintToUser("Network already stopped.")
			return nil
		}
		return fmt.Errorf("failed to stop network: %w", err)
	}
	ux.Logger.PrintToUser("Network stopped successfully.")

	if !dontSave {
		relayerConfigPath := app.GetAWMRelayerConfigPath()
		if utils.FileExists(relayerConfigPath) {
			relayerStoredConfigPath := filepath.Join(app.GetAWMRelayerSnapshotConfsDir(), snapshotName+jsonExt)
			if err := os.MkdirAll(filepath.Dir(relayerStoredConfigPath), constants.DefaultPerms755); err != nil {
				return err
			}
			if err := os.Rename(relayerConfigPath, relayerStoredConfigPath); err != nil {
				return fmt.Errorf("couldn't store relayer conf from %s into %s", relayerConfigPath, relayerStoredConfigPath)
			}
		}
		extraLocalNetworkDataPath := app.GetExtraLocalNetworkDataPath()
		if utils.FileExists(extraLocalNetworkDataPath) {
			storedExtraLocalNetowkrDataPath := filepath.Join(app.GetExtraLocalNetworkSnapshotsDir(), snapshotName+jsonExt)
			if err := os.MkdirAll(filepath.Dir(storedExtraLocalNetowkrDataPath), constants.DefaultPerms755); err != nil {
				return err
			}
			if err := os.Rename(extraLocalNetworkDataPath, storedExtraLocalNetowkrDataPath); err != nil {
				return fmt.Errorf("couldn't store extra local network data from %s into %s", extraLocalNetworkDataPath, storedExtraLocalNetowkrDataPath)
			}
		}
	}

	return nil
}
