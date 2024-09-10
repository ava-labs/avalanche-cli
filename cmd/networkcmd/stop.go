// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package networkcmd

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanche-cli/pkg/binutils"
	"github.com/ava-labs/avalanche-cli/pkg/cobrautils"
	"github.com/ava-labs/avalanche-cli/pkg/constants"
	"github.com/ava-labs/avalanche-cli/pkg/models"
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
	if err := stopAndSaveNetwork(dontSave); err != nil {
		if errors.Is(err, binutils.ErrGRPCTimeout) {
			// no server to kill
			return nil
		} else {
			return err
		}
	}

	var err error
	if err = binutils.KillgRPCServerProcess(app); err != nil {
		app.Log.Warn("failed killing server process", zap.Error(err))
		fmt.Println(err)
	} else {
		ux.Logger.PrintToUser("Server shutdown gracefully")
	}

	if err := teleporter.RelayerCleanup(
		app.GetLocalRelayerRunPath(models.Local),
		app.GetLocalRelayerStorageDir(models.Local),
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

	if _, err := cli.Status(ctx); err != nil {
		if server.IsServerError(err, server.ErrNotBootstrapped) {
			ux.Logger.PrintToUser("Network already stopped.")
			return nil
		}
		return fmt.Errorf("failed to get network status: %w", err)
	}

	autoSave := app.Conf.GetConfigBoolValue(constants.ConfigSnapshotsAutoSaveKey)

	if dontSave || autoSave {
		if _, err := cli.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop network: %w", err)
		}
		return nil
	} else {
		if _, err = cli.SaveSnapshot(ctx, snapshotName, true); err != nil {
			return fmt.Errorf("failed to stop network: %w", err)
		}
	}

	ux.Logger.PrintToUser("Network stopped successfully.")

	return nil
}
